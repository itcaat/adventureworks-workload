# AdventureWorks Workload: Research & Improvement Notes

Документ для будущей работы над `awload`. Сводка практик из экосистемы, сравнение с текущей реализацией и приоритетный roadmap.

## Цель проекта

Go-based workload generator для тестирования Microsoft SQL Server (в т.ч. cluster/AG). Имитация application traffic, а не валидация бизнес-логики AdventureWorks.

---

## Ландшафт: как делают workload на AdventureWorks

### Каноническая линия — BOL Random Workload

Почти все генераторы идут от одного источника:

1. **[Jonathan Kehayias — Random Workload Generator (2011)](https://www.sqlskills.com/blogs/jonathan/the-adventureworks2008r2-books-online-random-workload-generator/)**  
   Большой `.sql` с примерами SELECT из Books Online, разделённый delimiter'ом. PowerShell в бесконечном цикле берёт случайный запрос и выполняет через SMO. Для масштаба запускают **3–5 копий скрипта параллельно**.

2. **[Pieter Vanhove — Azure SQL workload (2016)](https://blogs.technet.microsoft.com/msftpietervanhove/2016/01/08/generate-workload-on-your-azure-sql-database/)**  
   Тот же подход, адаптированный под Azure SQL.

3. **[Rob Sewell — multi-threaded PowerShell](https://blog.robsewell.com/blog/generating-a-workload-against-adventureworks-with-powershell/)**  
   PoshRSJob: N jobs, throttle, **случайная пауза 100–2000 ms** между запросами.

4. **[Matticusau/SqlWorkloadGenerator](https://github.com/Matticusau/SqlWorkloadGenerator)** — ориентир проекта (см. README)  
   `RunWorkload.ps1` + `AdventureWorks*BOLWorkload.sql`, частоты Fast/Normal/Slow, несколько процессов для большей нагрузки.

5. **[Microsoft DP-300 lab](https://github.com/MicrosoftLearning/dp-300-database-administrator/blob/master/Instructions/Templates/CreateRandomWorkloadGenerator.sql)**  
   Тот же паттерн Kehayias (MIT), для учебных AG/failover сценариев.

**Общая идея:** не «реалистичное приложение», а **широкий пул разнообразных SELECT'ов** + **случайный выбор** + **конкурентность** + **think time**.

### Инструменты другого класса

| Инструмент | Назначение | Что взять |
|---|---|---|
| **[ostress (RML Utils)](https://learn.microsoft.com/en-us/troubleshoot/sql/tools/replay-markup-language-utility)** | N потоков × R итераций одного SQL | Постепенное наращивание `-n`, отдельные output dirs, query timeout `-t` |
| **[HammerDB TPROC-C](https://www.hammerdb.com/)** | Синтетический OLTP benchmark | Ramp-up 15–20+ мин, sustained load во время failover, reconnect после обрыва |
| **[Quest Benchmark Factory](https://support.quest.com/technical-documents/benchmark-factory-for-database/9.0/user-guide/2)** | Mixed workload с весами транзакций | Weighted mix, user scenarios (цепочки операций), scalability sweeps |
| **Distributed Replay** | Replay production trace | Deprecated в SQL Server 2022; Kehayias использовал BOL workload чтобы **сгенерировать trace** |

Для **cluster/AG testing** важнее не HammerDB (создаёт свою TPC-C схему), а **непрерывный application-like load + измерение reconnect/latency spike при failover**:

- [Failover walkthrough under load](https://www.nocentino.com/posts/2026-04-19-planned-failover-walkthrough-sql-server-kubernetes-operator/)
- [Microsoft: AG exceeded RTO](https://learn.microsoft.com/en-us/sql/database-engine/availability-groups/windows/troubleshoot-availability-group-exceeded-rto)
- [Readable secondaries and redo thread blocking](https://learn.microsoft.com/en-us/sql/database-engine/availability-groups/windows/troubleshoot-availability-group-exceeded-rto?view=sql-server-ver16#BKMK_REDOBLOCK)

---

## Что уже лучше в `awload`, чем в классике

| Классика (BOL/Matticusau) | `awload` |
|---|---|
| Случайный SELECT из 100+ запросов | 7 **осмысленных операций** (catalog, orders, inventory…) |
| Один тип «пользователя» | **Personas** (shopper, support, analyst, operations, hr) |
| Фиксированная частота | **Profiles** + weight modifiers |
| Нет метрик | **p50/p95/p99**, errors by operation, JSON/Markdown |
| Нет ramp-up | `-ramp` |
| Нет think time | `-think-min/max` × persona tempo |
| Write = хаос | Scoped **cart write** с префиксом `awload-` |
| Windows/PowerShell | **Go binary**, CI-friendly, cross-platform |
| Нет reproducibility | `-seed` |

По модели нагрузки `awload` ближе к **Benchmark Factory Mixed Workload**, чем к сырому BOL-генератору.

---

## Известные проблемы (smoke-read прогон)

### EXECUTE permission denied

| Операция | Объект | Причина |
|---|---|---|
| `customer_order_history` | `dbo.ufnGetSalesOrderStatusText` | scalar UDF требует `EXECUTE` |
| `employee_managers` | `EXEC dbo.uspGetEmployeeManagers` | stored proc требует `EXECUTE` |

Остальные операции работают под `db_datareader`, т.к. ходят только в таблицы.

**DB fix:**

```sql
USE AdventureWorks2022;
GRANT EXECUTE ON dbo.ufnGetSalesOrderStatusText TO [your_login];
GRANT EXECUTE ON dbo.uspGetEmployeeManagers TO [your_login];
```

**Code fix (предпочтительно для app-user):**

- `customer_order_history`: inline `CASE` вместо UDF
- `employee_managers`: inline recursive CTE вместо proc

BOL/Matticusau в основном используют SELECT на таблицы/views — без EXECUTE.

### context deadline exceeded (единичные ошибки)

Раньше при коротком прогоне (`-duration 30s`) в конце часто появлялись ошибки **shutdown race**: контекст `duration` отменял in-flight запросы, и они падали с `context deadline exceeded`, хотя p95 мог быть ~700ms.

**Сейчас:** после истечения `-duration` новые операции не стартуют, а активные запросы дорабатывают в рамках `-request-timeout`. Подробности — в README, раздел «Примечания по нагрузке».

Если ошибки остаются — это уже реальные таймауты запросов или проблемы с БД, а не обрыв по окончании прогона.

---

## Рекомендации по улучшению

### 1. Failover-aware клиент (приоритет для AG)

Сейчас ошибки попадают в `Failures` map без классификации. Для AG-тестов нужно:

- **Retry transient errors**: 40613, 40197, 40501, 10053/10054, communication link failure
- **Reconnect через listener** с `MultiSubnetFailover=true` в DSN
- **Отдельные метрики**: `reconnect_count`, `failover_gap_ms`, errors classified as `transient` vs `permanent`
- **Флаг `-applicationintent readonly`** для тестов readable secondary

### 2. Убрать зависимость от EXECUTE

Работать под `db_datareader` app-user. Inline SQL вместо UDF/proc (см. выше).

### 3. Заменить `ORDER BY NEWID()`

Используется в `customer_order_history`, `order_detail_lookup`, `vendor_purchasing` — full scan каждый раз.

Альтернативы:

- случайный ID из известного диапазона
- `TABLESAMPLE` / precomputed lookup table
- hot customers/orders, крутить по ним

### 4. Расширить portfolio операций (curated, не BOL dump)

Не копировать все 100+ BOL запросов. Добавить 5–10 ops с разным IO/CPU профилем:

| Операция | Источник в AdventureWorks | Зачем |
|---|---|---|
| `product_bom` | `uspGetBillOfMaterials` (inline) | recursive CTE, CPU |
| `sales_person_report` | view `Sales.vSalesPersonSalesByFiscalYears` | aggregation |
| `person_search` | `Person.Person` + `EmailAddress` | point lookup / index seek |
| `recent_orders` | `SalesOrderHeader` по `OrderDate` range | range scan, типичный OLTP |
| `credit_check` | `Sales.Customer` + `CreditLimit` | small read |

Matticusau даёт широту, `awload` — **realistic mix**. Лучше 12–15 curated ops с весами.

### 5. User scenarios (цепочки операций)

Сейчас каждая операция независима. Реальное приложение делает сценарии:

```
shopper:  catalog_search → catalog_search → cart_add_item
support:  customer_order_history → order_detail_lookup
```

Режим `-scenario-mode`: persona иногда выполняет **фиксированную цепочку 2–4 ops** — session affinity, plan cache reuse.

### 6. Протокол прогона для cluster tests

| Параметр | Smoke | Cluster/failover test |
|---|---|---|
| Duration | 30s–1m | **15–30m minimum** |
| Ramp | 5s | **2–5m** |
| Users | 5 | **50–200** |
| Measure window | весь прогон | warm-up 5m → measure 10m → failover → measure 10m |
| Connection | direct node | **AG listener** |
| Report | errors count | errors + reconnect + latency before/after failover marker |

Пример DSN для AG listener:

```
sqlserver://user:pass@ag-listener:1433?database=AdventureWorks2022&encrypt=true&TrustServerCertificate=true&MultiSubnetFailover=true
```

Read-only secondary:

```
...&ApplicationIntent=ReadOnly
```

### 7. Метрики и export

Из `.agents/progress.md`:

- Prometheus counters: `awload_ops_total{operation,status}`, `awload_latency_seconds`
- Time-series JSON: latency каждые N секунд
- CSV compare между прогонами для regression

### 8. Write workload — расширить осторожно

Cart-only write — правильный scoped подход. Опционально:

- read-your-writes сценарий: `cart_add_item` → immediate SELECT cart

Настоящие UPDATE/INSERT в production tables — anti-pattern (cleanup, side effects).

### 9. Документация

README утверждает «не требуют дополнительных процедур» — расходится с кодом. Дополнить:

- минимальные права: `db_datareader` (+ write на `ShoppingCartItem` для cart mode)
- рекомендуемый DSN для AG listener
- protocol для failover drill

---

## Позиционирование

```
                    Реалистичность приложения
                              ▲
                              │
         awload (с personas)   │    Production trace replay
              ★               │         (deprecated)
                              │
    BOL random queries ───────┼─────── ostress (script × N threads)
                              │
                              │    HammerDB TPC-C
                              ▼
                    Синтетический benchmark
```

**Ниша `awload`:** realistic enough + repeatable + measurable + AG-aware.

---

## Roadmap

### Quick wins

1. Inline SQL вместо UDF/proc → 0 permission errors под app-user
2. Random ID вместо `NEWID()` → стабильнее latency
3. Graceful shutdown → убрать shutdown noise в отчёте
4. README: AG listener DSN, минимальные grants

### Medium

5. Transient error retry + reconnect metrics
6. 3–5 новых read operations (views, range queries, recursive CTE)
7. `-applicationintent readonly` для secondary testing
8. Time-bucketed metrics в JSON

### Cluster drills

9. `-failover-marker` timestamp в report
10. Profile `failover-drill`: warm → sustained → документировать client-side gap
11. Prometheus export

---

## Что не стоит брать

- **Копировать весь BOL workload** — теряются personas/profiles, непредсказуемый mix
- **HammerDB как замену** — другая схема, не AdventureWorks
- **Distributed Replay** — deprecated, сложно
- **Запуск под sa** — скрывает permission problems, не похоже на prod

---

## Ссылки

- [Jonathan Kehayias — BOL Random Workload](https://www.sqlskills.com/blogs/jonathan/the-adventureworks2008r2-books-online-random-workload-generator/)
- [Rob Sewell — PowerShell workload](https://blog.robsewell.com/blog/generating-a-workload-against-adventureworks-with-powershell/)
- [Matticusau/SqlWorkloadGenerator](https://github.com/Matticusau/SqlWorkloadGenerator)
- [Microsoft DP-300 template](https://github.com/MicrosoftLearning/dp-300-database-administrator/blob/master/Instructions/Templates/CreateRandomWorkloadGenerator.sql)
- [ostress / RML Utils](https://learn.microsoft.com/en-us/troubleshoot/sql/tools/replay-markup-language-utility)
- [HammerDB](https://www.hammerdb.com/)
- [Distributed Replay (deprecated)](https://learn.microsoft.com/en-us/sql/tools/distributed-replay/sql-server-distributed-replay)
- [AG RTO troubleshooting](https://learn.microsoft.com/en-us/sql/database-engine/availability-groups/windows/troubleshoot-availability-group-exceeded-rto)

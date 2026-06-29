# AdventureWorks Workload Generator

Go-приложение для генерации реалистичной нагрузки на Microsoft SQL Server с базой AdventureWorks. Цель проекта не проверять бизнес-логику схемы, а имитировать работу обычного приложения: много виртуальных пользователей, разные поведенческие профили, запросы к продажам/каталогу/складу/поставщикам/HR и финальный отчет по производительности.

Ориентиром служит `Matticusau/SqlWorkloadGenerator`: там PowerShell запускает T-SQL пакеты с частотой и длительностью. Здесь та же идея перенесена в один Go-бинарь с управлением конкурентностью и метриками.

## Проверка общего сценария

Этот сценарий нужен, чтобы быстро убедиться, что приложение собирается, подключается к базе, выполняет чтение, выполняет опциональные записи и создает отчет.

1. Собрать бинарь и проверить CLI:

```bash
go mod tidy
go build ./cmd/awload
./awload -h
```

2. Задать подключение к SQL Server с восстановленной базой `AdventureWorks2022`:

```bash
export AWLOAD_DSN='sqlserver://sa:your-password@localhost:1433?database=AdventureWorks2022&encrypt=true&TrustServerCertificate=true'
```

3. Проверить read-only сценарий. Он не меняет данные в базе:

```bash
./awload \
  -dsn "$AWLOAD_DSN" \
  -users 5 \
  -duration 30s \
  -ramp 5s \
  -profile read-heavy \
  -report-name smoke-read
```

4. Проверить общий read/write сценарий. Записи включаются явно через `-write-mode cart` и используют только `Sales.ShoppingCartItem` со строками вида `awload-<run_id>-...`:

```bash
./awload \
  -dsn "$AWLOAD_DSN" \
  -users 10 \
  -duration 60s \
  -ramp 10s \
  -profile write-light \
  -write-mode cart \
  -report-name smoke-write
```

5. Проверить результат:

```bash
ls -la reports/
sed -n '1,80p' reports/smoke-read-*.md
sed -n '1,80p' reports/smoke-write-*.md
```

В успешном прогоне в отчете должны быть `Total operations` больше нуля, низкое или нулевое количество ошибок, latency p50/p95/p99 и строки по операциям `catalog_search`, `customer_order_history`, `sales_dashboard` и, для write-сценария, `cart_add_item` / `cart_update_item` / `cart_cleanup`.

## Возможности

- `-users`: количество одновременных виртуальных пользователей.
- Неодинаковое поведение пользователей: shopper, support, analyst, operations, hr.
- Профили нагрузки: `mixed`, `read-heavy`, `reporting`, `write-light`.
- Read-нагрузка по стандартным объектам AdventureWorks2022.
- Опциональная write-нагрузка через `Sales.ShoppingCartItem` с префиксом `awload-<run_id>`.
- Плавный старт пользователей через `-ramp`.
- Per-operation timeout.
- Финальный Markdown и JSON отчет в `reports/`.

## Быстрый старт

```bash
go mod tidy
go build ./cmd/awload
```

SQL authentication:

```bash
./awload \
  -server localhost:1433 \
  -database AdventureWorks2022 \
  -user sa \
  -password 'your-password' \
  -trust-server-cert \
  -users 50 \
  -duration 10m \
  -profile mixed
```

DSN напрямую:

```bash
./awload \
  -dsn 'sqlserver://sa:your-password@localhost:1433?database=AdventureWorks2022&encrypt=true&TrustServerCertificate=true' \
  -users 100 \
  -duration 30m \
  -profile read-heavy
```

Write-light сценарий нужно включать явно:

```bash
./awload \
  -dsn "$AWLOAD_DSN" \
  -users 40 \
  -duration 15m \
  -profile write-light \
  -write-mode cart
```

## Переменные окружения

Все основные параметры можно задать через env:

- `AWLOAD_DSN`
- `AWLOAD_SERVER`
- `AWLOAD_DATABASE`
- `AWLOAD_USER`
- `AWLOAD_PASSWORD`
- `AWLOAD_USERS`
- `AWLOAD_DURATION`
- `AWLOAD_PROFILE`
- `AWLOAD_WRITE_MODE`
- `AWLOAD_REPORT_DIR`

Флаги CLI имеют приоритет над значениями по умолчанию и env.

## Что генерируется

Read/proc/report операции:

- `catalog_search`: каталог товаров с категориями.
- `customer_order_history`: история заказов случайного клиента.
- `order_detail_lookup`: детализация случайного заказа.
- `inventory_availability`: остатки по складу.
- `sales_dashboard`: агрегаты продаж по территориям и годам.
- `vendor_purchasing`: заказы поставщиков.
- `employee_managers`: вызов `dbo.uspGetEmployeeManagers`.

Write операции при `-write-mode cart`:

- `cart_add_item`: вставляет строку корзины с валидным `ProductID`.
- `cart_update_item`: обновляет строки текущего запуска.
- `cart_cleanup`: удаляет часть строк текущего запуска старше 30 секунд.

## Отчеты

После запуска создаются:

- `reports/awload-<run_id>.md`
- `reports/awload-<run_id>.json`

В отчете есть throughput, p50/p95/p99, ошибки по операциям и распределение persona mix. JSON удобен для дальнейшей автоматизации и сравнения прогонов.

## Примечания по нагрузке

- Запросы используют стандартную схему AdventureWorks2022 и не требуют дополнительных процедур.
- В read-only режиме база не изменяется.
- В write-mode `cart` приложение пишет только в `Sales.ShoppingCartItem` и помечает свои строки префиксом `awload-`.
- Для named instance или нестандартной аутентификации лучше передавать полный `-dsn`.

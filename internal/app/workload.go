package app

import (
	"context"
	"database/sql"
	"fmt"
	"math/rand"
	"time"
)

type Operation struct {
	Name   string
	Kind   string
	Weight int
	Run    func(context.Context, *sql.DB, *rand.Rand, Persona) error
}

type Persona struct {
	ID             int
	Type           string
	Tempo          float64
	WeightModifier map[string]float64
}

func buildOperations(cfg Config, runID string) []Operation {
	ops := []Operation{
		{
			Name:   "catalog_search",
			Kind:   "read",
			Weight: 16,
			Run:    queryCatalogSearch,
		},
		{
			Name:   "customer_order_history",
			Kind:   "read",
			Weight: 18,
			Run:    queryCustomerOrderHistory,
		},
		{
			Name:   "order_detail_lookup",
			Kind:   "read",
			Weight: 14,
			Run:    queryOrderDetailLookup,
		},
		{
			Name:   "inventory_availability",
			Kind:   "read",
			Weight: 12,
			Run:    queryInventoryAvailability,
		},
		{
			Name:   "sales_dashboard",
			Kind:   "report",
			Weight: 9,
			Run:    querySalesDashboard,
		},
		{
			Name:   "vendor_purchasing",
			Kind:   "read",
			Weight: 7,
			Run:    queryVendorPurchasing,
		},
		{
			Name:   "employee_managers",
			Kind:   "proc",
			Weight: 5,
			Run:    queryEmployeeManagers,
		},
	}

	if cfg.WriteMode == "cart" {
		ops = append(ops,
			Operation{
				Name:   "cart_add_item",
				Kind:   "write",
				Weight: 5,
				Run:    cartAddItem(runID),
			},
			Operation{
				Name:   "cart_update_item",
				Kind:   "write",
				Weight: 3,
				Run:    cartUpdateItem(runID),
			},
			Operation{
				Name:   "cart_cleanup",
				Kind:   "write",
				Weight: 1,
				Run:    cartCleanup(runID),
			},
		)
	}

	profile, _ := profileWeights(cfg.Profile)
	filtered := ops[:0]
	for _, op := range ops {
		multiplier := profile[op.Kind]
		if multiplier <= 0 {
			continue
		}
		op.Weight = max(1, int(float64(op.Weight)*multiplier))
		filtered = append(filtered, op)
	}
	return filtered
}

func profileWeights(name string) (map[string]float64, bool) {
	switch name {
	case "mixed":
		return map[string]float64{"read": 1.0, "report": 1.0, "proc": 1.0, "write": 1.0}, true
	case "read-heavy":
		return map[string]float64{"read": 1.4, "report": 0.6, "proc": 0.8, "write": 0.3}, true
	case "reporting":
		return map[string]float64{"read": 0.6, "report": 2.5, "proc": 0.6, "write": 0}, true
	case "write-light":
		return map[string]float64{"read": 0.9, "report": 0.4, "proc": 0.5, "write": 2.2}, true
	default:
		return nil, false
	}
}

func newPersona(id int, rng *rand.Rand) Persona {
	types := []Persona{
		{Type: "shopper", Tempo: 0.75, WeightModifier: map[string]float64{"catalog_search": 1.8, "cart_add_item": 1.7, "customer_order_history": 0.7}},
		{Type: "support", Tempo: 1.05, WeightModifier: map[string]float64{"customer_order_history": 1.7, "order_detail_lookup": 1.6, "employee_managers": 0.5}},
		{Type: "analyst", Tempo: 1.45, WeightModifier: map[string]float64{"sales_dashboard": 2.4, "inventory_availability": 1.4, "catalog_search": 0.5}},
		{Type: "operations", Tempo: 0.95, WeightModifier: map[string]float64{"inventory_availability": 2.0, "vendor_purchasing": 1.7, "cart_cleanup": 1.5}},
		{Type: "hr", Tempo: 1.2, WeightModifier: map[string]float64{"employee_managers": 3.0, "sales_dashboard": 0.4}},
	}
	p := types[rng.Intn(len(types))]
	p.ID = id
	p.Tempo *= 0.7 + rng.Float64()*0.9
	return p
}

func chooseOperation(ops []Operation, p Persona, rng *rand.Rand) Operation {
	total := 0
	weights := make([]int, len(ops))
	for i, op := range ops {
		modifier := p.WeightModifier[op.Name]
		if modifier == 0 {
			modifier = 1
		}
		w := max(1, int(float64(op.Weight)*modifier))
		weights[i] = w
		total += w
	}

	target := rng.Intn(total)
	for i, w := range weights {
		if target < w {
			return ops[i]
		}
		target -= w
	}
	return ops[len(ops)-1]
}

func thinkDuration(cfg Config, p Persona, rng *rand.Rand) time.Duration {
	if cfg.ThinkMax == cfg.ThinkMin {
		return time.Duration(float64(cfg.ThinkMin) * p.Tempo)
	}
	delta := cfg.ThinkMax - cfg.ThinkMin
	return time.Duration(float64(cfg.ThinkMin+time.Duration(rng.Int63n(int64(delta)))) * p.Tempo)
}

func queryCatalogSearch(ctx context.Context, db *sql.DB, rng *rand.Rand, _ Persona) error {
	colors := []string{"", "Black", "Blue", "Red", "Silver", "Yellow", "Multi"}
	color := colors[rng.Intn(len(colors))]
	limit := 10 + rng.Intn(25)
	rows, err := db.QueryContext(ctx, `
SELECT TOP (@limit)
    p.ProductID,
    p.Name,
    p.ProductNumber,
    p.Color,
    p.ListPrice,
    ps.Name AS SubcategoryName,
    pc.Name AS CategoryName
FROM Production.Product AS p
LEFT JOIN Production.ProductSubcategory AS ps ON ps.ProductSubcategoryID = p.ProductSubcategoryID
LEFT JOIN Production.ProductCategory AS pc ON pc.ProductCategoryID = ps.ProductCategoryID
WHERE p.FinishedGoodsFlag = 1
  AND (@color = '' OR p.Color = @color)
ORDER BY p.ListPrice DESC, p.Name;`,
		sql.Named("limit", limit),
		sql.Named("color", color),
	)
	return drainRows(rows, err)
}

func queryCustomerOrderHistory(ctx context.Context, db *sql.DB, rng *rand.Rand, _ Persona) error {
	limit := 20 + rng.Intn(40)
	rows, err := db.QueryContext(ctx, `
WITH PickedCustomer AS (
    SELECT TOP (1) CustomerID
    FROM Sales.Customer
    ORDER BY NEWID()
)
SELECT TOP (@limit)
    c.CustomerID,
    p.FirstName,
    p.LastName,
    soh.SalesOrderID,
    soh.OrderDate,
    soh.TotalDue,
    dbo.ufnGetSalesOrderStatusText(soh.Status) AS StatusText
FROM PickedCustomer AS pc
JOIN Sales.Customer AS c ON c.CustomerID = pc.CustomerID
LEFT JOIN Person.Person AS p ON p.BusinessEntityID = c.PersonID
LEFT JOIN Sales.SalesOrderHeader AS soh ON soh.CustomerID = c.CustomerID
ORDER BY soh.OrderDate DESC;`,
		sql.Named("limit", limit),
	)
	return drainRows(rows, err)
}

func queryOrderDetailLookup(ctx context.Context, db *sql.DB, rng *rand.Rand, _ Persona) error {
	limit := 12 + rng.Intn(30)
	rows, err := db.QueryContext(ctx, `
WITH PickedOrder AS (
    SELECT TOP (1) SalesOrderID
    FROM Sales.SalesOrderHeader
    ORDER BY NEWID()
)
SELECT TOP (@limit)
    soh.SalesOrderID,
    soh.OrderDate,
    soh.TotalDue,
    sod.SalesOrderDetailID,
    sod.OrderQty,
    sod.UnitPrice,
    sod.LineTotal,
    p.Name AS ProductName
FROM PickedOrder AS po
JOIN Sales.SalesOrderHeader AS soh ON soh.SalesOrderID = po.SalesOrderID
JOIN Sales.SalesOrderDetail AS sod ON sod.SalesOrderID = soh.SalesOrderID
JOIN Production.Product AS p ON p.ProductID = sod.ProductID
ORDER BY sod.SalesOrderDetailID;`,
		sql.Named("limit", limit),
	)
	return drainRows(rows, err)
}

func queryInventoryAvailability(ctx context.Context, db *sql.DB, _ *rand.Rand, _ Persona) error {
	rows, err := db.QueryContext(ctx, `
SELECT TOP (50)
    p.ProductID,
    p.Name,
    SUM(pi.Quantity) AS QuantityOnHand,
    MIN(pi.Quantity) AS MinimumBinQuantity,
    MAX(pi.ModifiedDate) AS LastInventoryUpdate
FROM Production.Product AS p
JOIN Production.ProductInventory AS pi ON pi.ProductID = p.ProductID
GROUP BY p.ProductID, p.Name
ORDER BY QuantityOnHand ASC, p.Name;`)
	return drainRows(rows, err)
}

func querySalesDashboard(ctx context.Context, db *sql.DB, _ *rand.Rand, _ Persona) error {
	rows, err := db.QueryContext(ctx, `
SELECT
    st.Name AS Territory,
    YEAR(soh.OrderDate) AS OrderYear,
    COUNT_BIG(*) AS OrderCount,
    SUM(soh.TotalDue) AS Revenue,
    AVG(soh.TotalDue) AS AverageOrderValue
FROM Sales.SalesOrderHeader AS soh
LEFT JOIN Sales.SalesTerritory AS st ON st.TerritoryID = soh.TerritoryID
GROUP BY st.Name, YEAR(soh.OrderDate)
ORDER BY OrderYear DESC, Revenue DESC;`)
	return drainRows(rows, err)
}

func queryVendorPurchasing(ctx context.Context, db *sql.DB, rng *rand.Rand, _ Persona) error {
	limit := 10 + rng.Intn(20)
	rows, err := db.QueryContext(ctx, `
WITH PickedVendor AS (
    SELECT TOP (1) BusinessEntityID
    FROM Purchasing.Vendor
    ORDER BY NEWID()
)
SELECT TOP (@limit)
    v.BusinessEntityID,
    v.Name,
    poh.PurchaseOrderID,
    poh.OrderDate,
    poh.TotalDue,
    pod.OrderQty,
    pod.LineTotal
FROM PickedVendor AS pv
JOIN Purchasing.Vendor AS v ON v.BusinessEntityID = pv.BusinessEntityID
LEFT JOIN Purchasing.PurchaseOrderHeader AS poh ON poh.VendorID = v.BusinessEntityID
LEFT JOIN Purchasing.PurchaseOrderDetail AS pod ON pod.PurchaseOrderID = poh.PurchaseOrderID
ORDER BY poh.OrderDate DESC, pod.PurchaseOrderDetailID;`,
		sql.Named("limit", limit),
	)
	return drainRows(rows, err)
}

func queryEmployeeManagers(ctx context.Context, db *sql.DB, rng *rand.Rand, _ Persona) error {
	employeeID := 1 + rng.Intn(290)
	rows, err := db.QueryContext(ctx, `EXEC dbo.uspGetEmployeeManagers @BusinessEntityID = @employee_id;`,
		sql.Named("employee_id", employeeID),
	)
	return drainRows(rows, err)
}

func cartAddItem(runID string) func(context.Context, *sql.DB, *rand.Rand, Persona) error {
	return func(ctx context.Context, db *sql.DB, rng *rand.Rand, p Persona) error {
		cartID := fmt.Sprintf("awload-%s-u%03d-c%04d", runID, p.ID, rng.Intn(1000))
		quantity := 1 + rng.Intn(4)
		_, err := db.ExecContext(ctx, `
INSERT INTO Sales.ShoppingCartItem (ShoppingCartID, Quantity, ProductID, DateCreated, ModifiedDate)
SELECT @cart_id, @quantity, picked.ProductID, GETDATE(), GETDATE()
FROM (
    SELECT TOP (1) ProductID
    FROM Production.Product
    WHERE FinishedGoodsFlag = 1
    ORDER BY NEWID()
) AS picked;`,
			sql.Named("cart_id", cartID),
			sql.Named("quantity", quantity),
		)
		return err
	}
}

func cartUpdateItem(runID string) func(context.Context, *sql.DB, *rand.Rand, Persona) error {
	return func(ctx context.Context, db *sql.DB, rng *rand.Rand, p Persona) error {
		prefix := fmt.Sprintf("awload-%s-u%03d", runID, p.ID)
		quantity := 1 + rng.Intn(8)
		_, err := db.ExecContext(ctx, `
UPDATE TOP (1) Sales.ShoppingCartItem
SET Quantity = @quantity, ModifiedDate = GETDATE()
WHERE ShoppingCartID LIKE @prefix + '%';`,
			sql.Named("prefix", prefix),
			sql.Named("quantity", quantity),
		)
		return err
	}
}

func cartCleanup(runID string) func(context.Context, *sql.DB, *rand.Rand, Persona) error {
	return func(ctx context.Context, db *sql.DB, rng *rand.Rand, _ Persona) error {
		batch := 5 + rng.Intn(15)
		prefix := fmt.Sprintf("awload-%s", runID)
		_, err := db.ExecContext(ctx, `
DELETE TOP (@batch) FROM Sales.ShoppingCartItem
WHERE ShoppingCartID LIKE @prefix + '%'
  AND DateCreated < DATEADD(second, -30, GETDATE());`,
			sql.Named("batch", batch),
			sql.Named("prefix", prefix),
		)
		return err
	}
}

func drainRows(rows *sql.Rows, err error) error {
	if err != nil {
		return err
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return err
	}
	values := make([]sql.RawBytes, len(cols))
	scanArgs := make([]any, len(values))
	for i := range values {
		scanArgs[i] = &values[i]
	}
	for rows.Next() {
		if err := rows.Scan(scanArgs...); err != nil {
			return err
		}
	}
	return rows.Err()
}

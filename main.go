package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"sort"
	"strings"
	"time"

// Import the gopsutil package for CPU usage statistics
	"github.com/shirou/gopsutil/cpu"
// Import the gopsutil package for Disk usage statistics
	"github.com/shirou/gopsutil/disk"
// Import the gopsutil package for Memory usage statistics
	"github.com/shirou/gopsutil/mem"
// Import termui package to build terminal UI for application display
	ui "github.com/gizak/termui/v3"
// Import termui package to build terminal UI for application display
	"github.com/gizak/termui/v3/widgets"
	_ "github.com/go-sql-driver/mysql"
)

// Global variable declarations to store application state and metrics
var (
	db                   *sql.DB
	connectionCount      int
	replicationStatus    string
	hostname             string
	ipAddress            string
	dbVersion            string
	memoryUsage          string
	cpuUsage             string
	diskAvailable        string
	diskUsage            string
	binlogSizes          string
	dbCounts             string
	tableCounts          string
	slowQueries          string
	selectCount          int
	insertCount          int
	updateCount          int
	deleteCount          int
	topOperations        string
)

// main function initializes the application, parses flags, and begins setup
func main() {
// Define flags for user inputs such as database credentials and host information
	user := flag.String("user", "", "Database user")
// Define flags for user inputs such as database credentials and host information
	password := flag.String("password", "", "Database password")
// Define flags for user inputs such as database credentials and host information
	host := flag.String("host", "localhost", "Database host")
// Define flags for user inputs such as database credentials and host information
	database := flag.String("database", "", "Database name")
	flag.Parse()

	if *user == "" || *password == "" || *database == "" {
		log.Fatalln("Usage: go run main.go -user=<user> -password=<password> -host=<host> -database=<database>")
	}

	connectDB(*user, *password, *host, *database)
	defer db.Close()

	go collectMetrics()

	drawUI()
}

// connectDB function: Add a brief explanation of what this function does
func connectDB(user, password, host, database string) {
	var err error
	dsn := fmt.Sprintf("%s:%s@tcp(%s)/%s", user, password, host, database)
	db, err = sql.Open("mysql", dsn)
	if err != nil {
		log.Fatalf("Error connecting to the database: %v", err)
	}
	err = db.Ping()
	if err != nil {
		log.Fatalf("Error pinging the database: %v", err)
	}
	log.Println("Database connection successfully established")
}

// collectMetrics function: Add a brief explanation of what this function does
func collectMetrics() {
	for {
		cpuPercents, _ := cpu.Percent(0, false)
		cpuUsage = fmt.Sprintf("%.2f%%", cpuPercents[0])

		v, _ := mem.VirtualMemory()
		memoryUsage = fmt.Sprintf("%.2f%% used of %.2f GB", v.UsedPercent, float64(v.Total)/1024/1024/1024)

		err := db.QueryRow("SHOW STATUS LIKE 'Threads_connected'").Scan(new(string), &connectionCount)
		if err != nil {
			log.Println("Error fetching connection count:", err)
		}

		var slaveStatus string
		err = db.QueryRow("SHOW SLAVE STATUS").Scan(&slaveStatus)
		if err != nil {
			replicationStatus = "No replication"
		} else {
			replicationStatus = "Replication active"
		}

		err = db.QueryRow("SELECT VERSION()").Scan(&dbVersion)
		if err != nil {
			log.Println("Error fetching database version:", err)
		}

		hostname, err = os.Hostname()
		if err != nil {
			log.Println("Error fetching hostname:", err)
		}
		ipAddress, err = getLocalIP()
		if err != nil {
			log.Println("Error fetching IP address:", err)
		}

		usage, _ := disk.Usage("/")
		diskAvailable = fmt.Sprintf("%.2f GB available", float64(usage.Free)/1024/1024/1024)
		diskUsage = fmt.Sprintf("%.2f%% used", usage.UsedPercent)

		binlogSizes, _ = getLargestBinlogs(15)
		
		dbCounts, tableCounts, _ = getDatabaseAndTableCounts()
		
		slowQueries, _ = getTopSlowQueries()
		_, _ = getQueryCounts() // Esto actualizará selectCount, insertCount, etc.

		topOperations, _ = getTopOperations()

		time.Sleep(3 * time.Second)
	}
}

// getLocalIP function: Add a brief explanation of what this function does
func getLocalIP() (string, error) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "", err
	}
	for _, addr := range addrs {
		if ipNet, ok := addr.(*net.IPNet); ok && !ipNet.IP.IsLoopback() {
			if ipNet.IP.To4() != nil {
				return ipNet.IP.String(), nil
			}
		}
	}
	return "", fmt.Errorf("could not determine local IP address")
}

// getLargestBinlogs function: Add a brief explanation of what this function does
func getLargestBinlogs(limit int) (string, error) {
	rows, err := db.Query("SHOW BINARY LOGS")
	if err != nil {
		log.Println("Error fetching binlog sizes:", err)
		return "N/A", err
	}
	defer rows.Close()

	var logs []struct {
		Name string
		Size int64
	}

	for rows.Next() {
		var name string
		var size int64
		if err := rows.Scan(&name, &size); err == nil {
			logs = append(logs, struct {
				Name string
				Size int64
			}{Name: name, Size: size})
		}
	}

	sort.Slice(logs, func(i, j int) bool {
		return logs[i].Size > logs[j].Size
	})

	result := ""
	for i, log := range logs {
		if i >= limit {
			break
		}
		result += fmt.Sprintf("%s: %.2f MB\n", log.Name, float64(log.Size)/1024/1024)
	}
	return result, nil
}

// getDatabaseAndTableCounts function: Add a brief explanation of what this function does
func getDatabaseAndTableCounts() (string, string, error) {
	var dbCount int
	tableCounts := ""

	rows, err := db.Query("SHOW DATABASES")
	if err != nil {
		log.Println("Error fetching databases:", err)
		return "N/A", "N/A", err
	}
	defer rows.Close()

	var dbTables []struct {
		Name  string
		Count int
	}

	for rows.Next() {
		var dbName string
		if err := rows.Scan(&dbName); err == nil {
			if dbName != "information_schema" && dbName != "performance_schema" && dbName != "mysql" && dbName != "sys" {
				var tableCount int
				err := db.QueryRow(fmt.Sprintf("SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = '%s'", dbName)).Scan(&tableCount)
				if err == nil {
					dbTables = append(dbTables, struct {
						Name  string
						Count int
					}{Name: dbName, Count: tableCount})
				}
			}
		}
	}

	dbCount = len(dbTables)

	sort.Slice(dbTables, func(i, j int) bool {
		return dbTables[i].Count > dbTables[j].Count
	})

	for i, db := range dbTables {
		if i >= 15 {
			break
		}
		tableCounts += fmt.Sprintf("%s: %d tables\n", db.Name, db.Count)
	}

	return fmt.Sprintf("Total DBs: %d", dbCount), strings.TrimSpace(tableCounts), nil
}

// getTopSlowQueries function: Add a brief explanation of what this function does
func getTopSlowQueries() (string, error) {
	rows, err := db.Query(`
		SELECT id, user, time 
		FROM information_schema.processlist 
		WHERE command IN ('Query', 'Execute') 
		ORDER BY time DESC 
		LIMIT 5
	`)
	if err != nil {
		log.Println("Error fetching slow queries:", err)
		return "N/A", err
	}
	defer rows.Close()

	result := ""
	for rows.Next() {
		var id int
		var user string
		var time int
		if err := rows.Scan(&id, &user, &time); err == nil {
			result += fmt.Sprintf("ID: %d, User: %s, Time: %ds\n", id, user, time)
		}
	}
	return result, nil
}

// getQueryCounts function: Add a brief explanation of what this function does
func getQueryCounts() (string, error) {
	db.QueryRow("SHOW GLOBAL STATUS LIKE 'Com_select'").Scan(new(string), &selectCount)
	db.QueryRow("SHOW GLOBAL STATUS LIKE 'Com_insert'").Scan(new(string), &insertCount)
	db.QueryRow("SHOW GLOBAL STATUS LIKE 'Com_update'").Scan(new(string), &updateCount)
	db.QueryRow("SHOW GLOBAL STATUS LIKE 'Com_delete'").Scan(new(string), &deleteCount)

	return fmt.Sprintf("Select: %d, Insert: %d, Update: %d, Delete: %d", selectCount, insertCount, updateCount, deleteCount), nil
}

// Función para obtener el top 5 de operaciones más lentas
// getTopOperations function: Add a brief explanation of what this function does
func getTopOperations() (string, error) {
	rows, err := db.Query(`
		SELECT id, command, time, info
		FROM information_schema.processlist
		WHERE command IN ('Query', 'Execute')
		ORDER BY time DESC
		LIMIT 5
	`)
	if err != nil {
		log.Println("Error fetching top operations:", err)
		return "N/A", err
	}
	defer rows.Close()

	result := ""
	for rows.Next() {
		var id int
		var command string
		var time int
		var info string
		if err := rows.Scan(&id, &command, &time, &info); err == nil {
			tableName := extractTableName(info)
			result += fmt.Sprintf("ID: %d, Op: %s, Table: %s, Time: %ds\n", id, command, tableName, time)
		}
	}
	return result, nil
}

// Extraer el nombre de la tabla de la consulta SQL
// extractTableName function: Add a brief explanation of what this function does
func extractTableName(query string) string {
	words := strings.Fields(query)
	for i, word := range words {
		if strings.ToUpper(word) == "FROM" || strings.ToUpper(word) == "INTO" || strings.ToUpper(word) == "UPDATE" {
			if i+1 < len(words) {
				return words[i+1]
			}
		}
	}
	return "Unknown"
}

// Función drawUI con todos los cuadros de métricas configurados en sus posiciones
// drawUI function: Add a brief explanation of what this function does
func drawUI() {
	if err := ui.Init(); err != nil {
		log.Fatalf("Failed to initialize UI: %v", err)
	}
	defer ui.Close()

	// Define UI elements
	title := widgets.NewParagraph()
	title.Text = "MONITORING CONSOLE FOR MARIADB/MYSQL\nVer. 1.0 beta\nCreated by: Luis Contreras\nEmail: luis.contreras.do@gmail.com"
	title.TextStyle.Fg = ui.ColorWhite
	title.Border = false
	title.SetRect(0, 0, 100, 3)

	// Connection count
	connectionBox := widgets.NewParagraph()
	connectionBox.Title = "Connections"
	connectionBox.Text = fmt.Sprintf("%d", connectionCount)
	connectionBox.SetRect(0, 3, 25, 6)
	connectionBox.BorderStyle.Fg = ui.ColorYellow

	// Replication status
	replicationBox := widgets.NewParagraph()
	replicationBox.Title = "Replication"
	replicationBox.Text = replicationStatus
	replicationBox.SetRect(0, 6, 25, 9)
	replicationBox.BorderStyle.Fg = ui.ColorYellow

	// Hostname
	hostnameBox := widgets.NewParagraph()
	hostnameBox.Title = "Hostname"
	hostnameBox.Text = hostname
	hostnameBox.SetRect(0, 9, 25, 12)
	hostnameBox.BorderStyle.Fg = ui.ColorYellow

	// IP Address
	ipBox := widgets.NewParagraph()
	ipBox.Title = "IP Address"
	ipBox.Text = ipAddress
	ipBox.SetRect(0, 12, 25, 15)
	ipBox.BorderStyle.Fg = ui.ColorYellow

	// Database Version
	versionBox := widgets.NewParagraph()
	versionBox.Title = "DB Version"
	versionBox.Text = dbVersion
	versionBox.SetRect(0, 15, 25, 18)
	versionBox.BorderStyle.Fg = ui.ColorYellow

	// Memory Usage
	memoryBox := widgets.NewParagraph()
	memoryBox.Title = "Memory Usage"
	memoryBox.Text = memoryUsage
	memoryBox.SetRect(0, 18, 25, 21)
	memoryBox.BorderStyle.Fg = ui.ColorYellow

	// CPU Usage
	cpuBox := widgets.NewParagraph()
	cpuBox.Title = "CPU Usage"
	cpuBox.Text = cpuUsage
	cpuBox.SetRect(0, 21, 25, 24)
	cpuBox.BorderStyle.Fg = ui.ColorYellow

	// Disk Usage
	diskBox := widgets.NewParagraph()
	diskBox.Title = "Disk Usage"
	diskBox.Text = fmt.Sprintf("%s\n%s", diskAvailable, diskUsage)
	diskBox.SetRect(0, 24, 25, 27)
	diskBox.BorderStyle.Fg = ui.ColorYellow

	// Total DBs
	dbCountBox := widgets.NewParagraph()
	dbCountBox.Title = "Total DBs"
	dbCountBox.Text = dbCounts
	dbCountBox.SetRect(25, 3, 50, 6)
	dbCountBox.BorderStyle.Fg = ui.ColorYellow

	// Tables per DB (top 15)
	tableCountBox := widgets.NewParagraph()
	tableCountBox.Title = "Tables per DB"
	tableCountBox.Text = tableCounts
	tableCountBox.SetRect(25, 6, 50, 15)
	tableCountBox.BorderStyle.Fg = ui.ColorYellow

	// Top Binlogs
	binlogBox := widgets.NewParagraph()
	binlogBox.Title = "Top Binlogs"
	binlogBox.Text = binlogSizes
	binlogBox.SetRect(25, 15, 75, 24)
	binlogBox.BorderStyle.Fg = ui.ColorYellow

	// Individual Query Counts with adjusted width and centered text
	selectBox := widgets.NewParagraph()
	selectBox.Title = "Select Count"
	selectBox.Text = fmt.Sprintf("  %d  ", selectCount)
	selectBox.SetRect(50, 3, 66, 6)
	selectBox.BorderStyle.Fg = ui.ColorYellow

	insertBox := widgets.NewParagraph()
	insertBox.Title = "Insert Count"
	insertBox.Text = fmt.Sprintf("  %d  ", insertCount)
	insertBox.SetRect(66, 3, 82, 6)
	insertBox.BorderStyle.Fg = ui.ColorYellow

	updateBox := widgets.NewParagraph()
	updateBox.Title = "Update Count"
	updateBox.Text = fmt.Sprintf("  %d  ", updateCount)
	updateBox.SetRect(50, 6, 66, 9)
	updateBox.BorderStyle.Fg = ui.ColorYellow

	deleteBox := widgets.NewParagraph()
	deleteBox.Title = "Delete Count"
	deleteBox.Text = fmt.Sprintf("  %d  ", deleteCount)
	deleteBox.SetRect(66, 6, 82, 9)
	deleteBox.BorderStyle.Fg = ui.ColorYellow

	// Top 5 Operations
	topOperationsBox := widgets.NewParagraph()
	topOperationsBox.Title = "Top 5 Operations"
	topOperationsBox.Text = topOperations
	topOperationsBox.SetRect(25, 24, 75, 33)
	topOperationsBox.BorderStyle.Fg = ui.ColorYellow

	// Footer
	footer := widgets.NewParagraph()
	footer.Text = "Press Ctrl + C to exit\nCopyright 2024"
	footer.TextStyle.Fg = ui.ColorWhite
	footer.Border = false
	footer.SetRect(0, 33, 100, 36)

	// Initial Render
	ui.Render(
		title, connectionBox, replicationBox, hostnameBox, ipBox, versionBox,
		memoryBox, cpuBox, diskBox, dbCountBox, tableCountBox, binlogBox,
		selectBox, insertBox, updateBox, deleteBox, topOperationsBox, footer,
	)

	// Update loop
	ticker := time.NewTicker(3 * time.Second).C
	uiEvents := ui.PollEvents()

	for {
		select {
		case <-ticker:
			connectionBox.Text = fmt.Sprintf("%d", connectionCount)
			replicationBox.Text = replicationStatus
			hostnameBox.Text = hostname
			ipBox.Text = ipAddress
			versionBox.Text = dbVersion
			memoryBox.Text = memoryUsage
			cpuBox.Text = cpuUsage
			diskBox.Text = fmt.Sprintf("%s\n%s", diskAvailable, diskUsage)
			dbCountBox.Text = dbCounts
			tableCountBox.Text = tableCounts
			binlogBox.Text = binlogSizes
			selectBox.Text = fmt.Sprintf("  %d  ", selectCount)
			insertBox.Text = fmt.Sprintf("  %d  ", insertCount)
			updateBox.Text = fmt.Sprintf("  %d  ", updateCount)
			deleteBox.Text = fmt.Sprintf("  %d  ", deleteCount)
			topOperationsBox.Text = topOperations

			ui.Render(
				title, connectionBox, replicationBox, hostnameBox, ipBox, versionBox,
				memoryBox, cpuBox, diskBox, dbCountBox, tableCountBox, binlogBox,
				selectBox, insertBox, updateBox, deleteBox, topOperationsBox, footer,
			)

		case e := <-uiEvents:
			if e.Type == ui.KeyboardEvent {
				if e.ID == "q" || e.ID == "<C-c>" {
					return
				}
			}
		}
	}
}

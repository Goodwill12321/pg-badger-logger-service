# PostgreSQL Log Analyzer Service

This service provides a web interface for analyzing PostgreSQL logs using pgbadger. It allows you to:
- View a list of PostgreSQL servers
- Browse available log files for each server
- Generate pgbadger reports for selected log files
- View report generation status
- Access generated reports

## Prerequisites

- Go 1.21 or later
- PostgreSQL server(s) with logging enabled
- pgbadger (and perl) installed on the system

## Configuration

Create a `config/config.yaml` file with your PostgreSQL server details:

```yaml
servers:
  - name: "local_db"
    host: "localhost"
    port: 5432
    user: "postgres"
    password: "postgres"
    database: "postgres"
    sslmode: "disable"
  - name: "prod_db"
    host: "prod.example.com"
    port: 5432
    user: "postgres"
    password: "postgres"
    database: "postgres"
    sslmode: "disable"

report_dir: "./report"
```

## Installation

1. Clone the repository
2. Install dependencies:
   ```bash
   go mod tidy
   ```

## Running the Service

1. Start the service:
   ```bash
   go run main.go
   ```
   By default, the service uses `config/config.yaml`. You can specify a different config file:
   ```bash
   go run main.go -config /path/to/config.yaml
   ```

2. Open your browser and navigate to `http://localhost:8080`

## Features

1. **Server List**: The main page displays a list of configured PostgreSQL servers.

2. **Log Files**: Click on a server to view its available log files.

3. **Report Generation**: Click on a log file to start generating a pgbadger report.
   - The report will be saved in `report/<server_name>/<log_name>.html`
   - The generation process output is saved to `report/<server_name>/<log_name>.out`

4. **Report Status**: While a report is being generated:
   - You can view the generation progress
   - You can stop the generation process if needed
   - The system prevents multiple users from generating the same report simultaneously

5. **View Reports**: Access generated reports directly through the web interface

## API Endpoints

- `GET /api/servers` - List all configured servers
- `GET /api/logs/:server` - Get log files for a specific server
- `POST /api/report/:server` - Start report generation
- `GET /api/reports/:server` - List generated reports for a server
- `GET /api/report-status/:server/:report` - Get report generation status
- `POST /api/stop-report/:server/:report` - Stop report generation

## Report Format

Reports are generated using pgbadger with the following format:
```
pgbadger -v -f stderr --prefix "%t [%p]: [%l-1] [%d]" -o <output>.html -a 1 <input>.log
```

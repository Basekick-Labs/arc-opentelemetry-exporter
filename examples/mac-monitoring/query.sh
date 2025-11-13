#!/bin/bash
# Helper script to query Mac monitoring metrics from Arc

ARC_URL="http://localhost:8000"
DATABASE="metrics"

# Colors
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

function query() {
    local sql="$1"

    # Build the JSON payload
    local payload=$(jq -n --arg sql "$sql" '{sql: $sql, format: "json"}')

    # Build curl command with auth if needed
    if [ -n "$ARC_AUTH_TOKEN" ]; then
        curl -s -X POST "${ARC_URL}/api/v1/query" \
            -H "Content-Type: application/json" \
            -H "x-arc-database: ${DATABASE}" \
            -H "Authorization: Bearer $ARC_AUTH_TOKEN" \
            -d "$payload" | jq .
    else
        curl -s -X POST "${ARC_URL}/api/v1/query" \
            -H "Content-Type: application/json" \
            -H "x-arc-database: ${DATABASE}" \
            -d "$payload" | jq .
    fi
}

function header() {
    echo -e "\n${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${GREEN}$1${NC}"
    echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}\n"
}

# Check if Arc is running
if ! curl -s "${ARC_URL}/health" > /dev/null 2>&1; then
    echo -e "${YELLOW}⚠️  Arc is not running on ${ARC_URL}${NC}"
    exit 1
fi

case "${1:-dashboard}" in
    "dashboard")
        header "📊 Mac System Dashboard"

        echo -e "${YELLOW}🔍 Available Metrics Tables:${NC}"
        query "SHOW TABLES" | jq -r '.data[][]' | sort

        header "💻 CPU Usage (Last Minute)"
        query "SELECT
            labels->>'cpu' as cpu,
            labels->>'state' as state,
            round(avg(value)::numeric, 2) as avg_value
        FROM system_cpu_time
        WHERE time > now() - INTERVAL '1 minute'
        GROUP BY labels->>'cpu', labels->>'state'
        ORDER BY cpu, state
        LIMIT 20"

        header "🧠 Memory Usage"
        query "SELECT
            labels->>'state' as state,
            round((avg(value) / 1024 / 1024 / 1024)::numeric, 2) as avg_gb
        FROM system_memory_usage
        WHERE time > now() - INTERVAL '5 minutes'
        GROUP BY labels->>'state'"

        header "💾 Disk Usage"
        query "SELECT
            labels->>'device' as device,
            labels->>'mountpoint' as mountpoint,
            round((avg(value) / 1024 / 1024 / 1024)::numeric, 2) as used_gb
        FROM system_filesystem_usage
        WHERE time > now() - INTERVAL '5 minutes'
            AND labels->>'state' = 'used'
        GROUP BY labels->>'device', labels->>'mountpoint'
        LIMIT 10"

        header "📊 Load Average"
        query "SELECT
            time,
            value as load_1m
        FROM system_cpu_load_average_1m
        ORDER BY time DESC
        LIMIT 5"
        ;;

    "cpu")
        header "💻 CPU Metrics"
        query "SELECT
            time,
            labels->>'cpu' as cpu,
            labels->>'state' as state,
            round(value::numeric, 2) as value
        FROM system_cpu_time
        ORDER BY time DESC
        LIMIT 30"
        ;;

    "memory")
        header "🧠 Memory Metrics"
        query "SELECT
            time,
            labels->>'state' as state,
            round((value / 1024 / 1024 / 1024)::numeric, 2) as gb
        FROM system_memory_usage
        ORDER BY time DESC, state
        LIMIT 20"
        ;;

    "disk")
        header "💾 Disk I/O Metrics"
        query "SELECT
            time,
            labels->>'device' as device,
            labels->>'direction' as direction,
            round((value / 1024 / 1024)::numeric, 2) as mb
        FROM system_disk_io
        ORDER BY time DESC
        LIMIT 20"
        ;;

    "network")
        header "🌐 Network Metrics"
        query "SELECT
            time,
            labels->>'device' as device,
            labels->>'direction' as direction,
            round((value / 1024 / 1024)::numeric, 2) as mb
        FROM system_network_io
        ORDER BY time DESC
        LIMIT 20"
        ;;

    "tables")
        header "📋 Available Metrics Tables"
        query "SHOW TABLES"
        ;;

    "raw")
        if [ -z "$2" ]; then
            echo "Usage: $0 raw <table_name>"
            exit 1
        fi
        header "📊 Raw data from $2"
        query "SELECT * FROM $2 ORDER BY time DESC LIMIT 10"
        ;;

    "help"|"-h"|"--help")
        echo "Mac Monitoring Query Helper"
        echo ""
        echo "Usage: $0 [command]"
        echo ""
        echo "Commands:"
        echo "  dashboard     Show overall system dashboard (default)"
        echo "  cpu           Show CPU metrics"
        echo "  memory        Show memory metrics"
        echo "  disk          Show disk I/O metrics"
        echo "  network       Show network metrics"
        echo "  tables        List all available metrics tables"
        echo "  raw <table>   Show raw data from specific table"
        echo "  help          Show this help"
        echo ""
        echo "Examples:"
        echo "  $0                           # Show dashboard"
        echo "  $0 cpu                       # Show CPU metrics"
        echo "  $0 raw system_cpu_time       # Show raw CPU data"
        ;;

    *)
        echo "Unknown command: $1"
        echo "Run '$0 help' for usage information"
        exit 1
        ;;
esac

# 🚂 railflush

Automatically restart Railway deployments on a cron schedule. Prevents memory bloat from long-running services with memory leaks.

- **Tiny** — ~3-5MB Docker image (built from `scratch`)
- **Zero dependencies** — only Go stdlib
- **Near-zero cost** — runs as a cron job, container stops between runs
- **Fast** — restarts containers instantly (no rebuild or redeploy)

[![Deploy on Railway](https://railway.com/button.svg)](https://railway.com/deploy/JRSncj?referralCode=HI9hWz)

## How It Works

1. Railway triggers the container on a cron schedule (default: every 6 hours)
2. For each target service, railflush queries the Railway API for the latest active deployment
3. It triggers a `deploymentRestart` — this restarts the process inside the container without rebuilding
4. Logs results and exits

## Environment Variables

| Variable | Required | Default | Description |
|---|---|---|---|
| `RAILWAY_API_TOKEN` | Yes | — | API token from [railway.com/account/tokens](https://railway.com/account/tokens) |
| `SERVICE_IDS` | Yes | — | Comma-separated list of service IDs to restart |
| `PROJECT_ID` | No | Auto-detected via `RAILWAY_PROJECT_ID` | Railway project ID |
| `ENVIRONMENT_ID` | No | Auto-detected via `RAILWAY_ENVIRONMENT_ID` | Environment ID (e.g., production) |

When deployed in the same Railway project as your target services, `PROJECT_ID` and `ENVIRONMENT_ID` are automatically detected — you only need to set `RAILWAY_API_TOKEN` and `SERVICE_IDS`.

## Finding Service IDs

1. Open your Railway project dashboard
2. Click on the service you want to restart
3. Go to **Settings**
4. Copy the **Service ID** from the settings panel

Alternatively, the service ID is the UUID in the URL when viewing a service:
`https://railway.com/project/.../service/<SERVICE_ID>`

## Customizing the Schedule

Set the cron schedule in your Railway service settings under **Settings > Cron Schedule**:

| Schedule | Cron Expression |
|---|---|
| Every hour | `0 * * * *` |
| Every 6 hours | `0 */6 * * *` |
| Every 12 hours | `0 */12 * * *` |
| Daily at midnight UTC | `0 0 * * *` |
| Daily at 3 AM UTC | `0 3 * * *` |

## Usage Example

Set the following environment variables in your Railway service:

```
RAILWAY_API_TOKEN=your-api-token-here
SERVICE_IDS=service-id-1,service-id-2,service-id-3
```

Each cron run produces logs like:

```
🚂 railflush — restarting Railway deployments
📋 Targeting 3 service(s) in project abc123
🔍 Fetching latest deployment for service service-id-1
🔄 Restarting deployment dep-456 for service service-id-1
✅ Service service-id-1 restarted successfully
🔍 Fetching latest deployment for service service-id-2
🔄 Restarting deployment dep-789 for service service-id-2
✅ Service service-id-2 restarted successfully
🔍 Fetching latest deployment for service service-id-3
🔄 Restarting deployment dep-012 for service service-id-3
✅ Service service-id-3 restarted successfully
🏁 Done: 3 restarted, 0 failed (245ms)
```

## API Rate Limits

The service makes 2 API calls per target service (1 query + 1 restart mutation):

| Plan | Requests/Hour | Max Services per Run |
|---|---|---|
| Free | 100 | 50 |
| Hobby | 1,000 | 500 |
| Pro | 10,000 | 5,000 |

## License

[MIT](LICENSE)

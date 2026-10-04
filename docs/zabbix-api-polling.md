# Zabbix API polling integration

WatchAlert can poll active trigger problems from the Zabbix JSON-RPC API without a Webhook. The synchronizer runs only on the WatchAlert leader, enriches each problem with trigger and host data, and reconciles recoveries on every polling cycle.

## Prerequisites

1. Create a read-only Zabbix API user and API token. Grant it read access only to the host groups that WatchAlert should ingest.
2. Create a WatchAlert fault center and copy its ID.
3. Ensure WatchAlert can reach the Zabbix frontend endpoint `/api_jsonrpc.php` over HTTP or HTTPS.

## Create the datasource

Call the authenticated WatchAlert datasource API. `http.url` may point either to the Zabbix frontend base URL or directly to `api_jsonrpc.php`.

```http
POST /api/w8t/datasource/dataSourceCreate
Authorization: <WatchAlert token>
Content-Type: application/json
```

```json
{
  "name": "zabbix-production",
  "type": "Zabbix",
  "http": {
    "url": "https://zabbix.example.com/zabbix",
    "timeout": 10
  },
  "zabbixConfig": {
    "apiToken": "<read-only-zabbix-api-token>",
    "faultCenterId": "fc-xxxxxxxx",
    "pollInterval": 15,
    "timeout": 10,
    "initialLookback": 3600,
    "overlap": 300,
    "insecureSkipVerify": false,
    "severityMapping": {
      "5": "P0",
      "4": "P1",
      "3": "P2",
      "2": "P2",
      "1": "P2",
      "0": "P2"
    }
  },
  "labels": {
    "environment": "production"
  },
  "description": "Zabbix trigger problem polling",
  "enabled": true
}
```

The API token is returned as `******` by datasource list/get APIs. Sending an empty or masked token during an update keeps the stored token unchanged.

## Behavior

- `problem.get` runs every `pollInterval` seconds and is limited to trigger problems (`source=0`, `object=0`).
- `event.get` uses a persisted watermark plus `overlap` to backfill short-lived or downtime events. On first start it reads `initialLookback` seconds.
- `trigger.get` adds expressions, items, dependencies and host references.
- `host.get` adds interfaces, groups, templates, tags and inventory.
- A stable fingerprint is built from the datasource ID and the Zabbix problem event ID, so multiple-problem event generation does not collide.
- When a problem disappears, `event.get` resolves its recovery event and exact recovery time before WatchAlert records the recovery.
- Zabbix severity is mapped to `P0/P1/P2`; the original numeric and text severity remain in labels.

## Verification

After enabling the datasource:

1. Confirm the log contains `Zabbix 同步已启动` and subsequent `Zabbix 同步完成` entries.
2. Compare the number of active Zabbix trigger problems visible to the API user with the WatchAlert fault center count.
3. Create one test problem, acknowledge it, and recover it.
4. Confirm WatchAlert keeps one fingerprint for the lifecycle, shows the acknowledgement, sends the configured recovery notification, and writes one history event.

The current in-memory synchronization status is available from:

```http
GET /api/w8t/datasource/zabbixSyncStatus?id=<datasource-id>
```

Do not grant write access to the API token. This integration never acknowledges, closes, suppresses, or changes a Zabbix event.

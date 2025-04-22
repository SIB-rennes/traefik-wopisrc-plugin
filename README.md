# Traefik Query Sticky Plugin

> A Traefik middleware plugin that enables sticky sessions based on a dynamic query parameter (e.g. `WOPISrc`), storing sticky data in Redis. Designed for scenarios such as Collabora or other session-sensitive services.

## ✨ Features

- Sticky session based on a query parameter (configurable, e.g. `WOPISrc`)
- Reads/writes sticky cookie values to Redis
- Automatically strips outdated sticky cookies from the request
- Fully configurable through Docker labels or static config
- Flexible Redis integration and query parameter naming

---

## 🔧 Basic Configuration (Docker Compose)

```yaml
whoami:
  image: traefik/whoami
  hostname: "whoami-{{.Task.Slot}}"
  deploy:
    labels:
      - "traefik.enable=true"
      - "traefik.http.routers.whoami.rule=Host(`whoami.127.0.0.1.nip.io`)"
      - "traefik.http.routers.whoami.entrypoints=web"
      - "traefik.http.routers.whoami.middlewares=query-sticky@docker"
      - "traefik.http.services.whoami.loadbalancer.server.port=80"
      - "traefik.http.middlewares.query-sticky.plugin.traefik-query-sticky.cookieName=traefik_collabora_sticky"
      - "traefik.http.middlewares.query-sticky.plugin.traefik-query-sticky.redisAddr=redis:6379"
      - "traefik.http.middlewares.query-sticky.plugin.traefik-query-sticky.redisDb=0"
      - "traefik.http.middlewares.query-sticky.plugin.traefik-query-sticky.redisPassword="
      - "traefik.http.middlewares.query-sticky.plugin.traefik-query-sticky.redisConnectionTimeout=2"
      - "traefik.http.middlewares.query-sticky.plugin.traefik-query-sticky.queryName=WOPISrc"
```


⚙️ Plugin Options

Key | Type | Description | Required | Default
cookieName | string | Name of the sticky session cookie | No | traefik_collabora_sticky
redisAddr | string | Redis address | No | redis:6379
redisDb | uint | Redis database number | No | 0
redisPassword | string | Redis password (optional) | No | (empty)
redisConnectionTimeout | int | Timeout in seconds for Redis connection | No | 2
queryName | string | Query parameter used as the sticky key (e.g., WOPISrc) | ✅ Yes | (none)



🧩 Project Structure  
This plugin follows the standard Traefik v2 plugin layout. Your plugins-local folder should follow this structure:

```go
plugins-local/
  github.com/
    SIB-rennes/
      traefik-query-sticky/
        main.go
        internal/
          redis/
            redis.go
```

⚠️ Notes
This plugin only works with Redis. Make sure Traefik can reach your Redis instance.

The queryName parameter is mandatory. Without it, the plugin will not function.

Existing sticky cookies in the request are removed before being conditionally re-added from Redis.

A simple connection pool of 5 Redis clients is used.



👥 Authors  
Developed by SIB Rennes to support persistent sessions for services like Collabora through Traefik.


🛡️ License   
MIT — Free to use, modify, and integrate into your own projects.


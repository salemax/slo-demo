# [Backstage](https://backstage.io)

This is your newly scaffolded Backstage App, Good Luck!

To start the app, run:

```sh
yarn install
yarn start
```

## SLO plugin

`plugins/slo` adds an SLO card to an entity page: target, 28-day
compliance, error budget remaining and current burn rate, read from the
Prometheus recording rules through the Backstage proxy (D8 in
`docs/PLAN.md`). See `plugins/slo/README.md`.

The card needs the monitoring stack running, because that is what serves
Prometheus on `localhost:9090`:

```sh
docker compose -f ../monitoring/docker-compose.yml up -d --build
yarn install
yarn start
```

Grafana also listens on port 3000, so if you run the stack and Backstage at
the same time, give one of them another port (for Backstage, `app.listen.port`
and `app.baseUrl` in an `app-config.local.yaml`).

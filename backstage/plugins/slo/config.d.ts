export interface Config {
  /**
   * Configuration for the SLO entity card (plugins/slo).
   *
   * Where Prometheus lives is not here: that is the `/prometheus/api`
   * proxy endpoint, so the address stays in one place (D8).
   */
  slo?: {
    /**
     * Base url of the Grafana that serves the provisioned SLO dashboard,
     * e.g. http://localhost:3000. Used with the entity's
     * `slo-demo/grafana-dashboard-uid` annotation to build the card's
     * link. Omit it and the card renders without a link.
     *
     * @visibility frontend
     */
    grafanaBaseUrl?: string;
  };
}

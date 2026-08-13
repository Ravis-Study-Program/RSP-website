const responseCounts = new Map<string, number>();
let originRejections = 0;
let rateLimitRejections = 0;

export function recordResponse(status: number): void {
  const statusClass = `${Math.floor(status / 100)}xx`;
  responseCounts.set(statusClass, (responseCounts.get(statusClass) ?? 0) + 1);
}

export function recordOriginRejection(): void {
  originRejections += 1;
}

export function recordRateLimitRejection(): void {
  rateLimitRejections += 1;
}

export function renderMetrics(): string {
  const lines = [
    "# HELP rsp_auth_requests_total Auth HTTP responses by bounded status class.",
    "# TYPE rsp_auth_requests_total counter",
  ];
  for (const statusClass of ["2xx", "3xx", "4xx", "5xx"]) {
    lines.push(
      `rsp_auth_requests_total{status_class="${statusClass}"} ${responseCounts.get(statusClass) ?? 0}`,
    );
  }
  lines.push(
    "# HELP rsp_auth_origin_rejections_total Browser requests rejected by origin policy.",
    "# TYPE rsp_auth_origin_rejections_total counter",
    `rsp_auth_origin_rejections_total ${originRejections}`,
    "# HELP rsp_auth_rate_limit_rejections_total Requests rejected by the auth edge limiter.",
    "# TYPE rsp_auth_rate_limit_rejections_total counter",
    `rsp_auth_rate_limit_rejections_total ${rateLimitRejections}`,
  );
  return `${lines.join("\n")}\n`;
}

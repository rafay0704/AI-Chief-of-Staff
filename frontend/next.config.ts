import type { NextConfig } from "next";

// Proxy /api/* to the Go backend so the browser stays same-origin (no CORS).
// Override the target in other environments via API_PROXY_TARGET.
const target = process.env.API_PROXY_TARGET ?? "http://localhost:8080";

const nextConfig: NextConfig = {
  async rewrites() {
    return [{ source: "/api/:path*", destination: `${target}/:path*` }];
  },
};

export default nextConfig;

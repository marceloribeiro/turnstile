import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  // Self-contained server bundle for the self-hosted deploy (rsync + systemd).
  output: "standalone",
};

export default nextConfig;

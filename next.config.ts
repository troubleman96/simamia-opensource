import type { NextConfig } from 'next';

const nextConfig: NextConfig = {
  output: 'export',
  allowedDevOrigins: ['3000-il5qzxavvst1ip27p2538-a5afd258.us2.manus.computer'],
  trailingSlash: true,
  images: { unoptimized: true },
  poweredByHeader: false,
  turbopack: { root: process.cwd() }
};

export default nextConfig;

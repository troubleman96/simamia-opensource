export const SIMAMIA_API_BASE_URL = (
  process.env.NEXT_PUBLIC_SIMAMIA_API_URL ||
  process.env.NEXT_PUBLIC_HUDUMADESK_API_URL ||
  'http://127.0.0.1:8080/api/v1'
).replace(/\/+$/, '');

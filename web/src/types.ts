export interface HealthResponse {status:string;app:string;version:string;database:string}
export interface AppSettings {businessName:string;currencyCode:string;currencySymbol:string;setupComplete:boolean}
export interface RouterInfo {identity:string;version:string;model:string;boardName:string;uptime:string;cpu:string;cpuCount:string;freeMemory:string;totalMemory:string}

export interface HotSpotCounts {
  servers: number;
  profiles: number;
  users: number;
  activeSessions: number;
}
export interface HotSpotServer {
  name: string;
  interface: string;
  addressPool: string;
  profile: string;
  disabled: boolean;
}
export interface HotSpotProfile {
  id: string;
  name: string;
  rateLimit: string;
  sharedUsers: string;
  sessionTimeout: string;
  idleTimeout: string;
}
export interface HotSpotUser {
  id: string;
  name: string;
  profile: string;
  server: string;
  limitUptime: string;
  disabled: boolean;
  comment: string;
}
export interface HotSpotActiveSession {
  id: string;
  user: string;
  address: string;
  macAddress: string;
  server: string;
  uptime: string;
  bytesIn: string;
  bytesOut: string;
}
export interface HotSpotOverview {
  counts: HotSpotCounts;
  servers: HotSpotServer[];
  profiles: HotSpotProfile[];
  users: HotSpotUser[];
  activeSessions: HotSpotActiveSession[];
  previewLimit: number;
  truncated: boolean;
}

export interface VoucherBatch {
  id: number;
  reference: string;
  prefix: string;
  profileName: string;
  serverName: string;
  limitUptime: string;
  requestedCount: number;
  createdCount: number;
  failedCount: number;
  status: 'running' | 'completed' | 'partial' | 'failed';
  errorMessage?: string;
  createdAt: string;
}
export interface Voucher {
  id: number;
  batchId: number;
  username: string;
  profileName: string;
  serverName: string;
  limitUptime: string;
  status: 'available' | 'sold' | 'void';
  createdAt: string;
}
export interface VoucherBatchResult {
  batch: VoucherBatch;
  vouchers: Voucher[];
  untrackedUsernames?: string[];
  errorMessage?: string;
}

export interface VoucherProduct {
  id: number;
  name: string;
  profileName: string;
  serverName: string;
  limitUptime: string;
  priceMinor: number;
  active: boolean;
  stockCount: number;
  createdAt: string;
  updatedAt: string;
}
export interface POSSale {
  id: number;
  reference: string;
  totalMinor: number;
  currencyCode: string;
  paymentMethod: 'cash' | 'transfer' | 'card' | 'other';
  createdAt: string;
}
export interface POSSaleItem {
  id: number;
  voucherId: number;
  username: string;
  productId: number;
  productName: string;
  profileName: string;
  serverName: string;
  limitUptime: string;
  unitPriceMinor: number;
  currencyCode: string;
}
export interface POSSaleResult {
  sale: POSSale;
  items: POSSaleItem[];
}

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
  usernameLength: number;
  usernameMode: 'alphanumeric' | 'letters' | 'numbers';
  passwordMode: 'none' | 'same' | 'random';
  passwordLength: number;
  passwordCharset: 'alphanumeric' | 'letters' | 'numbers';
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
  password?: string;
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

export interface UserManagerLimitation {
  id: number;
  sourceId: string;
  name: string;
  properties: Record<string,string>;
  present: boolean;
  lastSeenAt: string;
}
export interface UserManagerProfileLimitation {
  position: number;
  properties: Record<string,string>;
  limitation: UserManagerLimitation;
}
export interface UserManagerProfile {
  id: number;
  routerId: number;
  sourceId: string;
  name: string;
  properties: Record<string,string>;
  present: boolean;
  lastSeenAt: string;
  limitations: UserManagerProfileLimitation[];
}
export interface UserManagerSyncSummary {
  routerId: number;
  routerKey: string;
  routerIdentity: string;
  routerOsVersion: string;
  model: string;
  profileCount: number;
  limitationCount: number;
  relationshipCount: number;
  syncedAt: string;
}
export interface UserManagerOverview {
  sync: UserManagerSyncSummary;
  router: RouterInfo;
  majorVersion: number;
  profiles: UserManagerProfile[];
  limitations: UserManagerLimitation[];
}

export interface ServiceProfileSyncSummary {
  routerId: number;
  routerKey: string;
  routerIdentity: string;
  routerOsVersion: string;
  model: string;
  sourceSystem: 'hotspot';
  profileCount: number;
  syncedAt: string;
}
export interface ServiceProfileRecord {
  id: number;
  routerId: number;
  sourceSystem: 'hotspot' | 'usermanager';
  sourceId: string;
  name: string;
  properties: Record<string,string>;
  present: boolean;
  lastSeenAt: string;
}
export interface HotSpotProfileSyncResponse {
  sync: ServiceProfileSyncSummary;
  router: RouterInfo;
  profiles: ServiceProfileRecord[];
}

export type ServiceSystem = 'hotspot' | 'usermanager';
export interface CardPackage {
  id: number;
  name: string;
  serviceSystem: ServiceSystem;
  serviceProfileId: number;
  serviceProfileName: string;
  serviceProfilePresent: boolean;
  routerId: number;
  routerAddress: string;
  routerIdentity: string;
  description: string;
  userPriceMinor: number;
  dealerPriceMinor: number;
  defaultPrefix: string;
  generationConfig: Record<string, unknown>;
  active: boolean;
  sortOrder: number;
  createdAt: string;
  updatedAt: string;
}
export interface CardPackageInput {
  name: string;
  serviceSystem: ServiceSystem;
  serviceProfileId: number;
  description: string;
  userPriceMinor: number;
  dealerPriceMinor: number;
  defaultPrefix: string;
  active: boolean;
  sortOrder: number;
}

export type PackageSelectionMode = 'all' | 'selected';
export interface PointOfSale {
  id: number;
  code: string;
  name: string;
  phone: string;
  address: string;
  notes: string;
  packageSelectionMode: PackageSelectionMode;
  active: boolean;
  packageRuleCount: number;
  createdAt: string;
  updatedAt: string;
}
export interface PointOfSaleInput {
  code: string;
  name: string;
  phone: string;
  address: string;
  notes: string;
  packageSelectionMode: PackageSelectionMode;
  active: boolean;
}
export interface PointOfSalePackageRule {
  packageId: number;
  packageName: string;
  serviceSystem: ServiceSystem;
  serviceProfileName: string;
  userPriceMinor: number;
  dealerPriceMinor: number;
  packageActive: boolean;
  isEnabled: boolean;
  hasRule: boolean;
  prefixOverride: string | null;
  templateIdOverride: number | null;
}
export interface PointOfSalePackageRuleInput {
  packageId: number;
  isEnabled: boolean;
  prefixOverride: string | null;
  templateIdOverride: number | null;
}
export interface PointOfSalePackagesResponse {
  pointOfSale: PointOfSale;
  packages: PointOfSalePackageRule[];
}

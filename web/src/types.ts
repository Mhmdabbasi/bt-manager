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

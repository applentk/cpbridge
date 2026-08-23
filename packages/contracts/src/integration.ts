import type { PlatformType } from './problem.js';

export interface PlatformIntegration {
  platform: PlatformType;
  externalUsername: string;
  connectionStatus: 'CONNECTED' | 'DISCONNECTED';
  updatedAt: string;
}

export interface UpdateIntegrationRequest {
  externalUsername: string;
  connectionStatus: 'CONNECTED' | 'DISCONNECTED';
}

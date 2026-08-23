import type { LanguageId, PlatformType } from './problem.js';

export type ExtensionErrorCode =
  | 'NOT_LOGGED_IN'
  | 'PLATFORM_UNAVAILABLE'
  | 'RATE_LIMITED'
  | 'UNSUPPORTED_LANGUAGE'
  | 'PROBLEM_NOT_FOUND'
  | 'SUBMISSION_FAILED'
  | 'UNKNOWN';

export interface ExtensionPingRequest {
  type: 'PING';
}

export interface ExtensionPingResponse {
  type: 'PONG';
  version: string;
  platforms: Partial<
    Record<
      PlatformType,
      {
        loggedIn: boolean;
        username?: string;
      }
    >
  >;
}

export interface ExtensionSubmitRequest {
  type: 'SUBMIT';
  submissionId: string;
  platform: PlatformType;
  problem: {
    externalId: string;
    url: string;
  };
  language: LanguageId;
  source: string;
}

export interface ExtensionSubmissionCreatedResponse {
  type: 'SUBMISSION_CREATED';
  submissionId: string;
  externalSubmissionId: string;
}

export interface ExtensionSubmissionFailedResponse {
  type: 'SUBMISSION_FAILED';
  submissionId: string;
  error: ExtensionErrorCode;
  message?: string;
}

export interface ExtensionStatusPollRequest {
  type: 'POLL_STATUS';
  platform: PlatformType;
  externalSubmissionId: string;
  problem: {
    externalId: string;
    url: string;
  };
}

export interface ExtensionStatusPollResponse {
  type: 'POLL_STATUS_RESULT';
  externalSubmissionId: string;
  status: 'JUDGING' | 'ACCEPTED' | 'WRONG_ANSWER' | 'TIME_LIMIT' | 'MEMORY_LIMIT' | 'RUNTIME_ERROR' | 'COMPILE_ERROR' | 'FAILED';
}

export type ExtensionMessage =
  | ExtensionPingRequest
  | ExtensionPingResponse
  | ExtensionSubmitRequest
  | ExtensionSubmissionCreatedResponse
  | ExtensionSubmissionFailedResponse
  | ExtensionStatusPollRequest
  | ExtensionStatusPollResponse;

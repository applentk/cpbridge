import type { LanguageId, PlatformType } from './problem.js';
export { LATEST_EXTENSION_VERSION } from './generated/extension-version.js';

export type ExtensionErrorCode =
  | 'NOT_LOGGED_IN'
  | 'PLATFORM_UNAVAILABLE'
  | 'INCOMPATIBLE_VERSION'
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

export interface ExtensionSubmissionActionRequiredResponse {
  type: 'SUBMISSION_ACTION_REQUIRED';
  submissionId: string;
  platform: 'CODEFORCES';
  action: 'COMPLETE_ANTIBOT';
  submitUrl: string;
  message: string;
}

export interface ExtensionCompleteManualSubmissionRequest {
  type: 'COMPLETE_MANUAL_SUBMISSION';
  submissionId: string;
}

export type StoredSubmissionDispatchState = 'DISPATCHING' | 'AWAITING_USER_ACTION' | 'CREATED' | 'FAILED';

export interface ExtensionRecoverSubmissionsRequest {
  type: 'RECOVER_SUBMISSIONS';
}

export interface ExtensionRecoveredSubmission {
  submissionId: string;
  state: StoredSubmissionDispatchState;
  externalSubmissionId?: string;
  error?: string;
  actionUrl?: string;
  actionMessage?: string;
}

export interface ExtensionRecoverSubmissionsResponse {
  type: 'RECOVER_SUBMISSIONS_RESULT';
  submissions: ExtensionRecoveredSubmission[];
}

export interface ExtensionAcknowledgeSubmissionRequest {
  type: 'ACK_SUBMISSION';
  submissionId: string;
}

export interface ExtensionAcknowledgeSubmissionResponse {
  type: 'ACK_SUBMISSION_RESULT';
  submissionId: string;
  acknowledged: boolean;
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
  | ExtensionSubmissionActionRequiredResponse
  | ExtensionCompleteManualSubmissionRequest
  | ExtensionRecoverSubmissionsRequest
  | ExtensionRecoverSubmissionsResponse
  | ExtensionAcknowledgeSubmissionRequest
  | ExtensionAcknowledgeSubmissionResponse
  | ExtensionStatusPollRequest
  | ExtensionStatusPollResponse;

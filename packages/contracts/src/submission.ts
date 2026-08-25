import type { LanguageId, PlatformType } from './problem.js';

export type SubmissionStatus =
  | 'PENDING'
  | 'DISPATCHING'
  | 'JUDGING'
  | 'ACCEPTED'
  | 'WRONG_ANSWER'
  | 'TIME_LIMIT'
  | 'MEMORY_LIMIT'
  | 'RUNTIME_ERROR'
  | 'COMPILE_ERROR'
  | 'FAILED';

export interface Submission {
  id: string;
  userId: string;
  problemId: string;
  contestId?: string | null;
  platform: PlatformType;
  language: LanguageId;
  sourceCode: string;
  status: SubmissionStatus;
  externalSubmissionId?: string | null;
  /** Official external submission page. Present only in admin API responses. */
  sourceUrl?: string | null;
  submittedAt: string;
  judgedAt?: string | null;
  metadata?: Record<string, unknown>;
  username?: string;
  problemTitle?: string;
}

export interface CreateSubmissionRequest {
  problemId: string;
  contestId?: string | null;
  language: LanguageId;
  sourceCode: string;
}

export interface UpdateSubmissionDispatchedRequest {
  externalSubmissionId: string;
}

export interface UpdateSubmissionResultRequest {
  status: SubmissionStatus;
  metadata?: Record<string, unknown>;
}

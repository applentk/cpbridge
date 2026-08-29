import type { PlatformType, Problem } from './problem.js';

export type Visibility = 'PUBLIC' | 'UNLISTED' | 'PRIVATE';

export interface ProblemSetItem {
  problemSetId: string;
  problemId: string;
  position: number;
  problem?: Problem;
}

export interface ProblemSet {
  id: string;
  ownerId: string;
  name: string;
  description: string;
  visibility: Visibility;
  createdAt: string;
  updatedAt: string;
  items?: ProblemSetItem[];
  problemCount?: number;
  ownerUsername?: string;
}

export interface CreateProblemSetRequest {
  name: string;
  description?: string;
  visibility?: Visibility;
}

export interface UpdateProblemSetRequest {
  name?: string;
  description?: string;
  visibility?: Visibility;
}

export interface AddProblemToSetRequest {
  problemId: string;
  position?: number;
}

export interface ReorderProblemSetRequest {
  problemIds: string[];
}

export interface ImportContestRequest {
  platform: PlatformType;
  contestUrl: string;
  name?: string;
  description?: string;
  visibility?: Visibility;
}

export interface ImportContestResult {
  problemSet: ProblemSet;
  problemCount: number;
  createdProblems: number;
  updatedProblems: number;
}

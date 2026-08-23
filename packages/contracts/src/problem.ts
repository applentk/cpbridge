export type PlatformType = 'CODEFORCES' | 'ATCODER';

export type LanguageId = 'cpp23' | 'python3' | 'java21' | 'go' | 'rust';

export interface SampleCase {
  input: string;
  output: string;
  explanation?: string;
}

export interface ProblemStatement {
  html: string;
  timeLimit?: string;
  memoryLimit?: string;
  sampleCases: SampleCase[];
}

export interface Problem {
  id: string;
  platform: PlatformType;
  externalId: string;
  title: string;
  url: string;
  difficulty: number | null;
  tags: string[];
  metadata: Record<string, any>;
  createdAt: string;
  updatedAt: string;
}

export interface ImportProblemRequest {
  url: string;
}

export interface CreateCustomProblemRequest {
  platform: PlatformType;
  externalId: string;
  title: string;
  url: string;
  difficulty?: number | null;
  tags?: string[];
  statement?: string;
  timeLimit?: string;
  memoryLimit?: string;
  sampleCases?: SampleCase[];
}

export interface ExtractStatementRequest {
  rawContent: string;
  platform?: PlatformType;
}

export interface ExtractStatementResponse {
  title?: string;
  statement: string;
  timeLimit?: string;
  memoryLimit?: string;
  sampleCases: SampleCase[];
}

export interface ProblemFilter {
  platform?: PlatformType;
  query?: string;
  minDifficulty?: number;
  maxDifficulty?: number;
  tag?: string;
  limit?: number;
  offset?: number;
}

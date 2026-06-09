export type User = {
  id: string;
  first_name: string;
  last_name: string;
  email: string;
  created_at: string;
};

export type Org = {
  id: string;
  name: string;
  website_url: string | null;
  created_at: string;
};

export type Member = {
  id: string;
  user_id: string;
  organization_id: string;
  role: string;
  created_at: string;
  first_name: string;
  last_name: string;
  email: string;
};

export type Invitation = {
  id: string;
  organization_id: string;
  email: string;
  role: string;
  status: string;
  created_at: string;
};

export type Project = {
  id: string;
  organization_id: string;
  name: string;
  description: string | null;
  created_at: string;
};

export type Deployment = {
  id: string;
  organization_id: string;
  project_id: string | null;
  name: string;
  ingest_key_prefix: string;
  last_seen_at: string | null;
  created_at: string;
  ingest_key?: string; // present only on creation
};

export type SessionRow = {
  id: string;
  deployment_id: string;
  project_id: string | null;
  session_key: string;
  source: string;
  user_label: string | null;
  model: string | null;
  requests: number;
  prompt_tokens: number;
  completion_tokens: number;
  cost: number;
  blocks: number;
  prevented: number;
  last_seen_at: string;
};

export type Summary = {
  sessions: number;
  requests: number;
  total_cost: number;
  blocks: number;
  dollars_prevented: number;
};

export type ModelUsage = {
  model: string;
  cost: number;
  requests: number;
  prompt_tokens: number;
  completion_tokens: number;
};

export type Usage = {
  sessions: number;
  requests: number;
  total_cost: number;
  dollars_prevented: number;
  by_model: ModelUsage[];
};

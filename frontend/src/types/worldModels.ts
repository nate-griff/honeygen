export interface Organization {
  name: string;
  description?: string;
  industry: string;
  size: string;
  region: string;
  domain_theme: string;
}

export interface Branding {
  tone: string;
  colors: string[];
}

export interface Employee {
  name: string;
  role: string;
  department: string;
}

export interface GenerationSettings {
  file_count_target: number;
  file_count_variance: number;
}

export interface WorldModelPayload {
  organization: Organization;
  branding: Branding;
  departments: string[];
  employees: Employee[];
  projects: string[];
  document_themes: string[];
  generation_settings?: GenerationSettings;
}

export interface WorldModelSummary {
  id: string;
  name: string;
  description: string;
  departmentCount: number;
  employeeCount: number;
  projectCount: number;
  documentThemeCount: number;
  createdAt: string;
  updatedAt: string;
}

export interface WorldModelDetails extends WorldModelPayload {
  id: string;
  name: string;
  description: string;
  createdAt: string;
  updatedAt: string;
}

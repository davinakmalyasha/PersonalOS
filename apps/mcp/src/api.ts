// Thin HTTP client over the Go API. No direct DB access, ever.

export class ApiError extends Error {
  constructor(
    public readonly status: number,
    public readonly code: string,
    message: string,
    public readonly details?: unknown,
  ) {
    super(message);
  }
}

export class PersonalOSClient {
  constructor(
    private readonly baseUrl: string,
    private readonly token: string | undefined,
  ) {}

  private headers(json = true): Record<string, string> {
    const h: Record<string, string> = {};
    if (json) h["Content-Type"] = "application/json";
    if (this.token) h["Authorization"] = `Bearer ${this.token}`;
    return h;
  }

  private async parseError(res: Response): Promise<never> {
    let code = "http_error";
    let message = `API ${res.status}`;
    let details: unknown;
    try {
      const body = (await res.json()) as { error?: string; code?: string; details?: unknown };
      message = body.error ?? message;
      code = body.code ?? code;
      details = body.details;
    } catch {
      /* non-JSON error body */
    }
    if (res.status === 401) {
      message =
        "unauthorized — is PERSONAL_OS_TOKEN set and matching the API's API_TOKEN? " + message;
    }
    throw new ApiError(res.status, code, message, details);
  }

  private async request<T>(
    method: string,
    path: string,
    query?: Record<string, unknown>,
    body?: unknown,
  ): Promise<T> {
    const url = new URL(path, this.baseUrl);
    if (query) {
      for (const [k, v] of Object.entries(query)) {
        if (v !== undefined && v !== null && v !== "") url.searchParams.set(k, String(v));
      }
    }
    const res = await fetch(url, {
      method,
      headers: this.headers(body !== undefined),
      body: body === undefined ? undefined : JSON.stringify(body),
    });
    if (!res.ok) await this.parseError(res);
    if (res.status === 204) return {} as T;
    return (await res.json()) as T;
  }

  get<T>(path: string, query?: Record<string, unknown>): Promise<T> {
    return this.request<T>("GET", path, query);
  }
  post<T>(path: string, body?: unknown): Promise<T> {
    return this.request<T>("POST", path, undefined, body ?? {});
  }
  patch<T>(path: string, body?: unknown): Promise<T> {
    return this.request<T>("PATCH", path, undefined, body ?? {});
  }
  del(path: string): Promise<void> {
    return this.request<void>("DELETE", path);
  }

  // CSV import is multipart/form-data.
  async importCsv(accountId: string, csvText: string, dateFormat?: string): Promise<unknown> {
    const form = new FormData();
    form.set("account_id", accountId);
    form.set("file", new Blob([csvText], { type: "text/csv" }), "statement.csv");
    if (dateFormat) form.set("date_format", dateFormat);
    const res = await fetch(new URL("/v1/transactions/import", this.baseUrl), {
      method: "POST",
      headers: this.headers(false),
      body: form,
    });
    if (!res.ok) await this.parseError(res);
    return res.json();
  }

  async health(): Promise<boolean> {
    try {
      const res = await fetch(new URL("/healthz", this.baseUrl));
      return res.ok;
    } catch {
      return false;
    }
  }
}

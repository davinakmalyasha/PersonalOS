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
  put<T>(path: string, body?: unknown): Promise<T> {
    return this.request<T>("PUT", path, undefined, body ?? {});
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

  // Receipt upload is multipart/form-data (phase 13b).
  async uploadReceipt(txnId: string, filename: string, bytes: Uint8Array): Promise<unknown> {
    const ext = filename.split(".").pop()?.toLowerCase() ?? "";
    const types: Record<string, string> = {
      pdf: "application/pdf", jpg: "image/jpeg", jpeg: "image/jpeg",
      png: "image/png", webp: "image/webp", heic: "image/heic",
    };
    const form = new FormData();
    form.set("file", new Blob([bytes as BlobPart], { type: types[ext] ?? "application/octet-stream" }), filename);
    const res = await fetch(new URL(`/v1/transactions/${txnId}/receipt`, this.baseUrl), {
      method: "POST",
      headers: this.headers(false),
      body: form,
    });
    if (!res.ok) await this.parseError(res);
    return res.json();
  }

  // Receipt download returns raw bytes + content type (phase 13b).
  async getReceipt(txnId: string): Promise<{ bytes: Uint8Array; contentType: string }> {
    const res = await fetch(new URL(`/v1/transactions/${txnId}/receipt`, this.baseUrl), {
      headers: this.headers(undefined),
    });
    if (!res.ok) await this.parseError(res);
    return { bytes: new Uint8Array(await res.arrayBuffer()), contentType: res.headers.get("content-type") ?? "" };
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

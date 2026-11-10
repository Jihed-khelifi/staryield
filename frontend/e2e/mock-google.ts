/**
 * A tiny OpenID Connect issuer standing in for Google in the end-to-end tests
 * (docs/platform-extensions.md §23.1). Node `http` + `node:crypto` only:
 *
 *   GET  /.well-known/openid-configuration   discovery document
 *   GET  /authorize                          302 straight back to `redirect_uri` with `code` + `state`
 *                                            (or `error=access_denied` after `denyNext()`)
 *   POST /token                              `client_secret_post`; verifies the PKCE S256 verifier
 *                                            and answers an RS256 `id_token`
 *   GET  /certs                              the JWKS
 *
 * The identity of the next sign-in is whatever the test selected with
 * `use()`, or `e2e_sub` / `e2e_email` query parameters on `/authorize`.
 */
import { createHash, generateKeyPairSync, randomBytes, sign, type KeyObject } from "node:crypto";
import { createServer, type IncomingMessage, type Server, type ServerResponse } from "node:http";

export type MockIdentity = { sub: string; email: string; email_verified?: boolean };

export type MockGoogleOptions = {
  port?: number;
  clientId?: string;
  clientSecret?: string;
};

type Grant = {
  clientId: string;
  redirectUri: string;
  nonce: string;
  challenge: string;
  identity: MockIdentity;
  expires: number;
};

const b64url = (input: Buffer | string) => Buffer.from(input).toString("base64url");

export class MockGoogle {
  readonly port: number;
  readonly issuer: string;
  readonly clientId: string;
  readonly clientSecret: string;
  /** Query parameters of the last /authorize request (for assertions on what the API asked for). */
  lastAuthorize: Record<string, string> | null = null;
  /** Number of /token requests answered successfully. */
  tokensIssued = 0;

  private server: Server | null = null;
  private readonly kid = randomBytes(8).toString("hex");
  private readonly privateKey: KeyObject;
  private readonly publicKey: KeyObject;
  private readonly grants = new Map<string, Grant>();
  private identity: MockIdentity = { sub: "e2e-default-sub", email: "e2e-default@example.com", email_verified: true };
  private deny = false;

  constructor(options: MockGoogleOptions = {}) {
    this.port = options.port ?? Number(process.env.E2E_GOOGLE_MOCK_PORT ?? 19090);
    this.issuer = `http://127.0.0.1:${this.port}`;
    this.clientId = options.clientId ?? "e2e-client";
    this.clientSecret = options.clientSecret ?? "e2e-secret";
    const pair = generateKeyPairSync("rsa", { modulusLength: 2048 });
    this.privateKey = pair.privateKey;
    this.publicKey = pair.publicKey;
  }

  /** The Google account the next sign-in is for. */
  use(identity: MockIdentity) {
    this.identity = { email_verified: true, ...identity };
  }

  /** The next /authorize answers like a visitor who pressed "Cancel" on Google's screen. */
  denyNext() {
    this.deny = true;
  }

  async start(): Promise<void> {
    if (this.server) return;
    const server = createServer((req, res) => {
      this.handle(req, res).catch((err) => {
        res.writeHead(500, { "Content-Type": "application/json" });
        res.end(JSON.stringify({ error: "server_error", error_description: String(err) }));
      });
    });
    await new Promise<void>((resolve, reject) => {
      server.once("error", reject);
      server.listen(this.port, "127.0.0.1", () => resolve());
    });
    this.server = server;
  }

  async stop(): Promise<void> {
    const server = this.server;
    this.server = null;
    if (!server) return;
    server.closeAllConnections?.();
    await new Promise<void>((resolve) => server.close(() => resolve()));
  }

  private json(res: ServerResponse, status: number, body: unknown, headers: Record<string, string> = {}) {
    res.writeHead(status, { "Content-Type": "application/json", "Cache-Control": "no-store", ...headers });
    res.end(JSON.stringify(body));
  }

  private async handle(req: IncomingMessage, res: ServerResponse) {
    const url = new URL(req.url ?? "/", this.issuer);
    if (req.method === "GET" && url.pathname === "/.well-known/openid-configuration") {
      return this.json(
        res,
        200,
        {
          issuer: this.issuer,
          authorization_endpoint: `${this.issuer}/authorize`,
          token_endpoint: `${this.issuer}/token`,
          jwks_uri: `${this.issuer}/certs`,
          response_types_supported: ["code"],
          subject_types_supported: ["public"],
          id_token_signing_alg_values_supported: ["RS256"],
          scopes_supported: ["openid", "email"],
          token_endpoint_auth_methods_supported: ["client_secret_post"],
          code_challenge_methods_supported: ["S256"],
          claims_supported: ["iss", "aud", "sub", "email", "email_verified", "nonce", "iat", "exp"],
        },
        { "Cache-Control": "public, max-age=3600" },
      );
    }
    if (req.method === "GET" && url.pathname === "/certs") {
      const jwk = this.publicKey.export({ format: "jwk" });
      return this.json(res, 200, { keys: [{ ...jwk, kid: this.kid, alg: "RS256", use: "sig" }] }, { "Cache-Control": "public, max-age=3600" });
    }
    if (req.method === "GET" && url.pathname === "/authorize") return this.authorize(url, res);
    if (req.method === "POST" && url.pathname === "/token") return this.token(req, res);
    return this.json(res, 404, { error: "not_found" });
  }

  private authorize(url: URL, res: ServerResponse) {
    const q = Object.fromEntries(url.searchParams.entries());
    this.lastAuthorize = q;
    const redirect = q.redirect_uri;
    if (q.client_id !== this.clientId || !redirect || !q.state) {
      return this.json(res, 400, { error: "invalid_request", error_description: "unknown client, or missing redirect_uri / state" });
    }
    let target: URL;
    try {
      target = new URL(redirect);
    } catch {
      return this.json(res, 400, { error: "invalid_request", error_description: "bad redirect_uri" });
    }
    target.searchParams.set("state", q.state);
    if (this.deny) {
      this.deny = false;
      target.searchParams.set("error", "access_denied");
    } else {
      if (q.response_type !== "code" || q.code_challenge_method !== "S256" || !q.code_challenge || !q.nonce) {
        target.searchParams.set("error", "invalid_request");
        res.writeHead(302, { Location: target.toString(), "Cache-Control": "no-store" });
        return res.end();
      }
      const identity: MockIdentity = q.e2e_sub || q.e2e_email ? { ...this.identity, sub: q.e2e_sub || this.identity.sub, email: q.e2e_email || this.identity.email } : this.identity;
      const code = randomBytes(24).toString("base64url");
      this.grants.set(code, { clientId: q.client_id, redirectUri: redirect, nonce: q.nonce, challenge: q.code_challenge, identity, expires: Date.now() + 5 * 60_000 });
      target.searchParams.set("code", code);
    }
    res.writeHead(302, { Location: target.toString(), "Cache-Control": "no-store" });
    res.end();
  }

  private async token(req: IncomingMessage, res: ServerResponse) {
    const chunks: Buffer[] = [];
    for await (const chunk of req) chunks.push(chunk as Buffer);
    const form = new URLSearchParams(Buffer.concat(chunks).toString("utf8"));
    const fail = (error: string, description: string) => this.json(res, 400, { error, error_description: description });

    if (form.get("grant_type") !== "authorization_code") return fail("unsupported_grant_type", "authorization_code only");
    // client_secret_post: credentials travel in the form body.
    if (form.get("client_id") !== this.clientId || form.get("client_secret") !== this.clientSecret) {
      return this.json(res, 401, { error: "invalid_client", error_description: "bad client credentials (client_secret_post)" });
    }
    const code = form.get("code") ?? "";
    const grant = this.grants.get(code);
    // A code works once.
    this.grants.delete(code);
    if (!grant || grant.expires < Date.now()) return fail("invalid_grant", "unknown, used or expired code");
    if (form.get("redirect_uri") !== grant.redirectUri) return fail("invalid_grant", "redirect_uri mismatch");
    const verifier = form.get("code_verifier") ?? "";
    if (!/^[A-Za-z0-9._~-]{43,128}$/.test(verifier)) return fail("invalid_grant", "code_verifier has the wrong shape");
    if (createHash("sha256").update(verifier).digest("base64url") !== grant.challenge) return fail("invalid_grant", "PKCE verification failed");

    const now = Math.floor(Date.now() / 1000);
    const claims = {
      iss: this.issuer,
      aud: grant.clientId,
      azp: grant.clientId,
      sub: grant.identity.sub,
      email: grant.identity.email,
      email_verified: grant.identity.email_verified ?? true,
      nonce: grant.nonce,
      iat: now,
      exp: now + 3600,
    };
    const signingInput = `${b64url(JSON.stringify({ alg: "RS256", typ: "JWT", kid: this.kid }))}.${b64url(JSON.stringify(claims))}`;
    const signature = sign("RSA-SHA256", Buffer.from(signingInput), this.privateKey);
    this.tokensIssued += 1;
    return this.json(res, 200, {
      access_token: randomBytes(16).toString("base64url"),
      token_type: "Bearer",
      expires_in: 3600,
      scope: "openid https://www.googleapis.com/auth/userinfo.email",
      id_token: `${signingInput}.${b64url(signature)}`,
    });
  }
}

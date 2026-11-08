import { createHash, generateKeyPairSync, randomBytes, sign } from 'node:crypto';
import { createServer, type IncomingMessage, type Server, type ServerResponse } from 'node:http';

/**
 * A minimal OpenID Connect provider for the sign-in end-to-end tests:
 * discovery, JWKS, an authorization page with one button per test user and
 * a token endpoint that checks the client, the redirect URI and the PKCE
 * verifier before it hands out an RS256-signed ID token.
 *
 * The ports and client settings are shared with playwright.config.ts, which
 * starts a second Go-RED instance configured against this provider.
 */

export const OIDC_SERVER_PORT = Number(process.env.GO_RED_E2E_OIDC_PORT || 8082);
export const OIDC_PROVIDER_PORT = Number(process.env.GO_RED_E2E_IDP_PORT || 8083);
export const OIDC_SERVER_URL = `http://localhost:${OIDC_SERVER_PORT}`;
export const OIDC_ISSUER = `http://localhost:${OIDC_PROVIDER_PORT}`;
export const OIDC_CLIENT_ID = 'go-red-e2e';
export const OIDC_CLIENT_SECRET = 'e2e-client-secret';
export const OIDC_ALLOWED_EMAIL = 'ada@example.com';

export interface TestUser {
  sub: string;
  email: string;
  name: string;
}

export const USERS: Record<string, TestUser> = {
  ada: { sub: 'user-ada', email: OIDC_ALLOWED_EMAIL, name: 'Ada Lovelace' },
  eve: { sub: 'user-eve', email: 'eve@example.org', name: 'Eve Example' },
};

interface Authorization {
  redirectUri: string;
  state: string;
  nonce: string;
  challenge: string;
}

interface Grant extends Authorization {
  user: TestUser;
}

const b64url = (data: Buffer | string) => Buffer.from(data).toString('base64url');
const token = () => randomBytes(16).toString('base64url');

export interface FakeProvider {
  /** How often the token endpoint issued an ID token. */
  issued: () => number;
  close: () => Promise<void>;
}

/** Starts the provider on OIDC_PROVIDER_PORT. */
export async function startOidcProvider(): Promise<FakeProvider> {
  const { privateKey, publicKey } = generateKeyPairSync('rsa', { modulusLength: 2048 });
  const jwk = { ...publicKey.export({ format: 'jwk' }), kid: 'e2e', alg: 'RS256', use: 'sig' };
  const pending = new Map<string, Authorization>(); // authorization page id -> request
  const grants = new Map<string, Grant>(); // code -> grant
  let issued = 0;

  const json = (res: ServerResponse, status: number, body: unknown) => {
    res.writeHead(status, { 'Content-Type': 'application/json', 'Cache-Control': 'no-store' });
    res.end(JSON.stringify(body));
  };

  const idToken = (grant: Grant) => {
    const now = Math.floor(Date.now() / 1000);
    const header = b64url(JSON.stringify({ alg: 'RS256', typ: 'JWT', kid: 'e2e' }));
    const payload = b64url(
      JSON.stringify({
        iss: OIDC_ISSUER,
        aud: OIDC_CLIENT_ID,
        sub: grant.user.sub,
        email: grant.user.email,
        email_verified: true,
        name: grant.user.name,
        nonce: grant.nonce,
        iat: now,
        exp: now + 300,
      })
    );
    const signature = sign('sha256', Buffer.from(`${header}.${payload}`), privateKey);
    return `${header}.${payload}.${b64url(signature)}`;
  };

  const readBody = (req: IncomingMessage) =>
    new Promise<string>((resolve) => {
      let body = '';
      req.on('data', (chunk) => (body += chunk));
      req.on('end', () => resolve(body));
    });

  const handle = async (req: IncomingMessage, res: ServerResponse) => {
    const url = new URL(req.url ?? '/', OIDC_ISSUER);
    const q = url.searchParams;

    if (req.method === 'GET' && url.pathname === '/.well-known/openid-configuration') {
      return json(res, 200, {
        issuer: OIDC_ISSUER,
        authorization_endpoint: `${OIDC_ISSUER}/authorize`,
        token_endpoint: `${OIDC_ISSUER}/token`,
        jwks_uri: `${OIDC_ISSUER}/keys`,
        response_types_supported: ['code'],
        subject_types_supported: ['public'],
        id_token_signing_alg_values_supported: ['RS256'],
        code_challenge_methods_supported: ['S256'],
      });
    }
    if (req.method === 'GET' && url.pathname === '/keys') {
      return json(res, 200, { keys: [jwk] });
    }
    if (req.method === 'GET' && url.pathname === '/authorize') {
      const ok =
        q.get('client_id') === OIDC_CLIENT_ID &&
        q.get('response_type') === 'code' &&
        q.get('code_challenge_method') === 'S256' &&
        (q.get('scope') ?? '').split(' ').includes('openid') &&
        !!q.get('redirect_uri') &&
        !!q.get('state') &&
        !!q.get('nonce') &&
        !!q.get('code_challenge');
      if (!ok) {
        res.writeHead(400, { 'Content-Type': 'text/plain' });
        return res.end('invalid authorization request');
      }
      const id = token();
      pending.set(id, { redirectUri: q.get('redirect_uri')!, state: q.get('state')!, nonce: q.get('nonce')!, challenge: q.get('code_challenge')! });
      const buttons = Object.entries(USERS)
        .map(([key, user]) => `<p><a data-testid="idp-user-${key}" href="/approve?id=${id}&amp;user=${key}">Sign in as ${user.name}</a></p>`)
        .join('');
      res.writeHead(200, { 'Content-Type': 'text/html; charset=utf-8' });
      return res.end(`<!doctype html><title>Fake provider</title><h1>Fake provider</h1>${buttons}`);
    }
    if (req.method === 'GET' && url.pathname === '/approve') {
      const request = pending.get(q.get('id') ?? '');
      const user = USERS[q.get('user') ?? ''];
      if (!request || !user) {
        res.writeHead(400, { 'Content-Type': 'text/plain' });
        return res.end('unknown authorization');
      }
      pending.delete(q.get('id')!);
      const code = token();
      grants.set(code, { ...request, user });
      const target = new URL(request.redirectUri);
      target.searchParams.set('code', code);
      target.searchParams.set('state', request.state);
      res.writeHead(302, { Location: target.toString() });
      return res.end();
    }
    if (req.method === 'POST' && url.pathname === '/token') {
      const form = new URLSearchParams(await readBody(req));
      // x/oauth2 sends the client credentials as HTTP Basic or in the form.
      let clientId = form.get('client_id');
      let secret = form.get('client_secret');
      const basic = req.headers.authorization;
      if (basic?.startsWith('Basic ')) {
        const [id, pass] = Buffer.from(basic.slice(6), 'base64').toString().split(':');
        clientId = decodeURIComponent(id);
        secret = decodeURIComponent(pass ?? '');
      }
      if (clientId !== OIDC_CLIENT_ID || secret !== OIDC_CLIENT_SECRET) return json(res, 401, { error: 'invalid_client' });

      const grant = grants.get(form.get('code') ?? '');
      grants.delete(form.get('code') ?? ''); // codes are single use
      const verifier = form.get('code_verifier') ?? '';
      if (
        !grant ||
        form.get('grant_type') !== 'authorization_code' ||
        form.get('redirect_uri') !== grant.redirectUri ||
        createHash('sha256').update(verifier).digest('base64url') !== grant.challenge
      ) {
        return json(res, 400, { error: 'invalid_grant' });
      }
      issued++;
      return json(res, 200, { access_token: token(), token_type: 'Bearer', expires_in: 300, id_token: idToken(grant) });
    }
    res.writeHead(404, { 'Content-Type': 'text/plain' });
    res.end('not found');
  };

  const server: Server = createServer((req, res) => {
    handle(req, res).catch(() => {
      res.writeHead(500);
      res.end();
    });
  });
  await new Promise<void>((resolve, reject) => {
    server.once('error', reject);
    server.listen(OIDC_PROVIDER_PORT, '127.0.0.1', resolve);
  });
  return {
    issued: () => issued,
    close: () =>
      new Promise<void>((resolve) => {
        server.closeAllConnections();
        server.close(() => resolve());
      }),
  };
}

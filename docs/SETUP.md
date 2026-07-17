# Setup: Google Cloud OAuth client

`unspool` talks to the YouTube Data API v3 as *you*, using your own Google Cloud OAuth client.
There is no shared client bundled with the tool — that would mean shipping a secret in a public
repo, which Google's terms don't allow for a data-scoped app. Each user creates their own,
free, one-time.

## 1. Create a project and enable the API

```bash
./scripts/setup-gcp.sh
```

This creates (or reuses) a Google Cloud project and enables the YouTube Data API v3 on it. It
is idempotent — safe to re-run.

## 2. Configure the OAuth consent screen

Google has no scriptable path for this step. The setup script prints a direct link when it
finishes; otherwise:

1. Open **APIs & Services → OAuth consent screen** in the
   [Cloud Console](https://console.cloud.google.com/auth/overview).
2. Choose **External** (unless you have a Google Workspace org and prefer Internal).
3. Fill in an app name (e.g. "unspool") and your email as the developer contact.
4. Add your own Google account under **Test users** — while the app is in "Testing" status,
   only test users can authenticate. This is fine for personal use; there's no need to submit
   for verification.

## 3. Create an OAuth client ID

1. Open **APIs & Services → Credentials → Create Credentials → OAuth client ID**.
2. Application type: **Desktop app**.
3. Name it anything (e.g. "unspool CLI").
4. Click **Create**, then **Download JSON** on the resulting client.

## 4. Install the client secret

Save the downloaded file to:

```text
~/.config/unspool/client_secret.json
```

(Or point `oauth_client_secret_file` in `~/.config/unspool/config.toml` at a different path.)

## 5. Log in

```bash
unspool --login
```

This opens your browser for the Google consent screen, then stores a refresh token in your
system keychain. You won't need to repeat this unless you revoke access or log out
(`unspool --logout`).

## Headless / SSH use

The login flow needs a browser on the same host as the loopback callback
(`http://127.0.0.1:<port>`). Over SSH with no local browser, `unspool --login` will time out
after 3 minutes rather than hang. Two options:

- **Port-forward** the loopback port from your local machine, then re-run `--login` on the
  remote host.
- **Log in locally once**, then point `store_dir` (and the keychain, which is host-local — see
  note below) so the remote host reuses credentials. In practice this usually means running
  `--login` directly on each host you use, since the OAuth token lives in that host's system
  keychain, not in `store_dir`.

## Troubleshooting

- **"redirect_uri_mismatch"** — Desktop app clients accept any loopback redirect URI
  automatically; this error usually means the downloaded JSON is for a different client type
  (e.g. "Web application"). Recreate it as a Desktop app client.
- **"Access blocked: this app's request is invalid"** — your Google account isn't listed as a
  test user on the consent screen (step 2.4).
- **Quota errors** — the YouTube Data API v3 free tier is 10,000 units/day per project; see the
  main README for how `unspool` budgets against it.

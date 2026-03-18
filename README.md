# vaultenv

**Securely share `.env` files with your team.** vaultenv lets your team securely sync environment variables using GitHub for authentication and client-side encryption. No servers to run, no cloud accounts to set up. Vaulenv is completely free, just install and go.

## Why vaultenv?

Every dev team has the same problem: you need to share database URLs, API keys, and other secrets. People end up pasting secrets in Slack DMs, emailing `.env` files, or committing them to private repos in plaintext. Paid alternatives exist that require cloud accounts and per developer fees.

vaultenv offers:

- **Encrypted end-to-end**: Secrets are encrypted on your machine before they ever leave it. GitHub stores only encrypted blobs (inside of a private repo) so even GitHub admins can't read your secrets.
- **Zero infrastructure**: No servers, no cloud accounts, no databases. Everything runs through GitHub repos you already have.
- **Team-friendly**: The vault owner controls who has access. New team members request access, the owner approves, and they're in.
- **Works with your workflow**: Push and pull `.env` files like you push and pull code. Optional git hooks auto-sync on every `git push`.
- **CI/CD ready**: Create scoped deployment keys so your pipelines can pull secrets without a human in the loop.
- **Portable**: Log in from any machine with your GitHub account and vault passphrase. Your encryption keys travel with you.

## Supported Platforms

| OS      | Architecture          | Binary                       |
| ------- | --------------------- | ---------------------------- |
| Linux   | x86_64 (amd64)        | `vaultenv-linux-amd64`       |
| Linux   | ARM64 (aarch64)       | `vaultenv-linux-arm64`       |
| macOS   | Intel (amd64)         | `vaultenv-darwin-amd64`      |
| macOS   | Apple Silicon (arm64) | `vaultenv-darwin-arm64`      |
| Windows | x86_64 (amd64)        | `vaultenv-windows-amd64.exe` |
| Windows | ARM64                 | `vaultenv-windows-arm64.exe` |

## Install

One command (Linux and macOS):

```bash
curl -fsSL https://raw.githubusercontent.com/Barestack-io/vaultenv/main/install.sh | sh
```

You'll be prompted to choose between a system-wide install (`/usr/local/bin`, requires sudo) or a user-only install (`~/.local/bin`). To skip the prompt:

```bash
# System-wide (recommended, requires sudo)
curl -fsSL https://raw.githubusercontent.com/Barestack-io/vaultenv/main/install.sh | sh -s -- --global

# Current user only (no sudo)
curl -fsSL https://raw.githubusercontent.com/Barestack-io/vaultenv/main/install.sh | sh -s -- --user
```

**Other install methods:**

```bash
# Via Go (requires Go toolchain)
go install github.com/Barestack-io/vaultenv/cmd/vaultenv@latest

# Build from source
git clone https://github.com/Barestack-io/vaultenv.git
cd vaultenv
make build        # builds for your current platform
make build-all    # cross-compile for all 6 platforms
```

**Windows**: Download the `.exe` binary from the [latest release](https://github.com/Barestack-io/vaultenv/releases/latest) and add it to your PATH. NOTE: We have not tested vaultenv on windows. YMMV!

## Quick Start

This is all it takes to get your team sharing secrets securely:

```bash
# Step 1: Log in with your GitHub account
vaultenv login
# Opens your browser -- authorize vaultenv, then set a vault passphrase

# Step 2: Link your project
cd ~/projects/my-api
vaultenv link
# Detects your git remote, finds or creates the org vault, and links

# Step 3: Push your staging env to the team
vaultenv push staging
# Reads .env.staging, encrypts it, and uploads to the vault

# OR Push your  personal .env file to the vault (see Personal Environments below)
vaultenv push

# Step 4: A teammate pulls it on their machine
vaultenv pull staging
# Downloads, decrypts, and writes .env.staging locally
```

That's it. Your `.env.staging` is now encrypted in GitHub and only your authorized team members can decrypt it.

### Personal Environments

Every developer has their own local overrides -- database ports, debug flags, personal API keys. vaultenv handles this too:

```bash
# Push your personal .env (only YOU can decrypt this, not even the vault owner)
vaultenv push

# Pull your personal .env on another machine
vaultenv pull
```

Personal environments are stored in the same org vault but encrypted exclusively for you.

## How It Works Under the Hood

### Authentication

vaultenv uses GitHub's [device flow](https://docs.github.com/en/apps/oauth-apps/building-oauth-apps/authorizing-oauth-apps#device-flow) -- the same mechanism the official GitHub CLI (`gh`) uses. When you run `vaultenv login`:

1. vaultenv requests a one-time code from GitHub
2. You visit github.com/login/device and enter the code
3. vaultenv polls GitHub until you approve
4. You get an access token -- no client secret is needed (the device flow is designed for CLI tools)

The token is stored locally at `~/.config/vaultenv/config.json` and is scoped to `repo` access (needed to read/write the private vault repo).

### Storage

vaultenv uses **private GitHub repos** as its storage backend -- no external services:

- **Personal vault** (`<your-username>/vaultenv-secrets`): Created automatically on first login. Stores only your encrypted private key and public key. This is how vaultenv finds your keys when you log in from a new machine.

- **Org vault** (`<your-org>/vaultenv-secrets`): One per GitHub organization. Stores encrypted `.env` files for all repos in the org, organized by project. Created automatically when the first project is linked.

```
<org>/vaultenv-secrets/                  # Private GitHub repo
  .vaultenv-repo.json                    # Marker identifying this as a vaultenv vault
  my-api/                                # One folder per project
    vault.json                           # Access control: owner, approved users, deploy keys
    environments/
      shared/
        staging.enc                      # Encrypted .env.staging
        staging.json                     # Key envelopes (one per authorized user)
        production.enc
        production.json
      personal/
        alice.enc                        # Alice's personal .env (only she can decrypt)
        alice.json
```

### Encryption and Key Management

vaultenv uses **hybrid NaCl envelope encryption** -- the same cryptographic primitives used by tools like [age](https://age-encryption.org/) and Signal.

**Per-user keypair (X25519 / Curve25519):**
- Generated on first login
- The private key is encrypted with your vault passphrase using Argon2id (a memory-hard key derivation function resistant to brute-force attacks) and stored in your personal vault repo
- The public key is shared with your team via the org vault

**When you push an environment file:**
1. A random 32-byte symmetric key is generated
2. The `.env` file content is encrypted with this key using NaCl `secretbox` (XSalsa20-Poly1305)
3. The symmetric key is then wrapped (encrypted) separately for each authorized user using NaCl `box` (Curve25519 key exchange + XSalsa20-Poly1305)
4. The encrypted file and the per-user key envelopes are uploaded to the vault repo

**When you pull an environment file:**
1. The encrypted file and your key envelope are downloaded
2. Your private key unwraps the symmetric key from your envelope
3. The symmetric key decrypts the file content
4. The plaintext `.env` file is written locally

This means each user can only decrypt files they've been explicitly authorized for. Even if someone gets read access to the entire vault repo, the encrypted blobs are useless without the corresponding private keys.

**Key portability**: Your private key is encrypted with your vault passphrase (Argon2id + secretbox) and stored in your personal vault. When you `vaultenv login` on a new machine, it downloads the encrypted key and asks for your passphrase. The underlying X25519 keypair never changes, so changing your passphrase only requires re-encrypting the key blob -- no re-encryption of any environment files.

## Security Considerations

Before using vaultenv, understand the security model:

**What vaultenv protects against:**
- Secrets exposed in plaintext in repos, chat messages, or emails
- Unauthorized team members accessing secrets they shouldn't see
- CI/CD pipelines accessing environments they shouldn't (deployment keys are scoped)
- Someone reading the vault repo without the encryption keys

**What vaultenv does NOT protect against:**
- **Compromised GitHub account**: If an attacker gains access to your GitHub account AND knows your vault passphrase, they can decrypt your secrets. Use a strong GitHub password, enable 2FA, and choose a strong vault passphrase.
- **Compromised local machine**: If an attacker has access to your machine, they can read your local `.env` files and your decrypted private key at `~/.config/vaultenv/keys/`. This is true for any secret management tool.
- **GitHub platform compromise**: While GitHub never sees your plaintext secrets (encryption is client-side), a sufficiently motivated attacker with access to GitHub's infrastructure could theoretically modify the vault repo contents. The encryption prevents reading, not tampering.
- **Weak vault passphrase**: Your vault passphrase protects your private key. vaultenv enforces minimum requirements (12+ characters, uppercase, digit, special character), but choose something strong.

**Recommendations:**
- Enable two-factor authentication on your GitHub account
- Choose a vault passphrase you don't use anywhere else
- Rotate deployment keys periodically and revoke any that are no longer needed
- Use environment-scoped deployment keys (don't give your staging CI pipeline access to production secrets)
- Keep `.env`, `.env.*`, and `.vaultenv` in your `.gitignore` (the `link` command does this automatically)

## Complete Command Reference

### `vaultenv login`

Authenticate with GitHub and set up your encryption keys.

```bash
vaultenv login
```

**What it does:**
1. Starts the GitHub device flow -- displays a code and URL
2. Opens your browser to github.com/login/device
3. After you approve, stores the access token locally
4. If this is your first time: generates an X25519 keypair, asks you to create a vault passphrase, encrypts the private key, creates your personal vault repo, and uploads the encrypted key
5. If you've used vaultenv before: downloads your encrypted private key from your personal vault and asks for your passphrase to decrypt it

**Passphrase requirements:** Minimum 12 characters, at least one uppercase letter, one digit, and one special character.

---

### `vaultenv init <namespace>`

Pre-create a vault repo for an organization. This is for org admins who want to set up the vault before team members start linking projects.

```bash
vaultenv init my-org
```

Most users don't need this -- `vaultenv link` creates the vault automatically if you have repo creation permissions in the org.

---

### `vaultenv link`

Link the current git repository to vaultenv. Run this inside a cloned repo with a GitHub remote.

```bash
cd ~/projects/my-api
vaultenv link
```

**What it does:**
1. Checks that you're logged in (runs `login` if not)
2. Reads the git remote URL to determine the org and repo name
3. Finds or creates the org vault repo (`<org>/vaultenv-secrets`)
4. If this project has no vault config yet: creates one and makes you the owner
5. If a vault config exists and you're authorized: links your local project
6. If you're not authorized: offers to submit an access request to the vault owner
7. Adds `.env`, `.env.*`, and `.vaultenv` to `.gitignore` if missing
8. Offers to install a git pre-push hook that auto-syncs your personal `.env`

**Vault naming**: The default vault repo name is `<org>/vaultenv-secrets`. If that name is already taken by a non-vaultenv repo, it falls back to `<org>/vaultenv-vault`. If both are taken, you'll be prompted to choose a custom name.

---

### `vaultenv push [environment]`

Encrypt and upload a `.env` file to the vault.

```bash
# Push a shared environment (encrypted for all authorized users)
vaultenv push staging        # reads .env.staging, encrypts, uploads
vaultenv push production     # reads .env.production, encrypts, uploads

# Push your personal .env (encrypted for you only)
vaultenv push                # reads .env, encrypts for you only, uploads
```

**Shared environments** are encrypted for every approved user and every deployment key scoped to that environment. When new users are authorized, the vault owner re-encrypts shared environments to include them.

**Personal environments** are encrypted with only your public key. No one else -- not even the vault owner -- can decrypt them.

---

### `vaultenv pull [environment]`

Download and decrypt a `.env` file from the vault.

```bash
# Pull a shared environment
vaultenv pull staging        # downloads, decrypts, writes .env.staging

# Pull your personal .env
vaultenv pull                # downloads, decrypts, writes .env
```

**Flags:**

| Flag                   | Description                                                                                                                                             |
| ---------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `--export`             | Print `export KEY=VALUE` lines to stdout instead of writing a file. Use with `eval $(vaultenv pull staging --export)` to inject into the current shell. |
| `--format github-env`  | Append `KEY=VALUE` lines to the file at `$GITHUB_ENV` (for GitHub Actions).                                                                             |
| `--deploy-key <token>` | Use a deployment key token directly (overrides the `VAULTENV_DEPLOY_KEY` env var).                                                                      |

**CI/CD mode**: When the `VAULTENV_DEPLOY_KEY` environment variable is set, `pull` operates non-interactively. It decodes the deployment key token, uses `GITHUB_TOKEN` or `VAULTENV_GITHUB_TOKEN` for vault repo access, and requires no local config files.

---

### `vaultenv authorize`

Approve pending access requests. Only the vault owner can run this.

```bash
vaultenv authorize
```

Lists all pending access requests for the current project. You can approve all or select specific users by entering comma-separated usernames. When a user is approved, all shared environment symmetric keys are re-encrypted to include their public key.

---

### `vaultenv deploy-key create <name>`

Create a deployment key for CI/CD pipelines. You must be an authorized user (owner or approved).

```bash
# Create a key scoped to staging only
vaultenv deploy-key create ci-staging --environments staging

# Create a key for multiple environments
vaultenv deploy-key create ci-all --environments staging,production

# Create a key for all shared environments (default)
vaultenv deploy-key create ci-everything
```

**Flags:**

| Flag                         | Description                                                                                     |
| ---------------------------- | ----------------------------------------------------------------------------------------------- |
| `--environments <env1,env2>` | Comma-separated list of environments this key can decrypt. Defaults to all shared environments. |

**Output**: Prints a deployment key token (a long opaque string starting with `vaultenv_dk_v1_`). This token is shown **once** and cannot be retrieved again -- the private key is not stored anywhere in the vault. Store it immediately as a CI/CD secret.

---

### `vaultenv deploy-key list`

List all deployment keys for the current project.

```bash
vaultenv deploy-key list
```

Shows each key's name, scoped environments, who created it, and when.

---

### `vaultenv deploy-key revoke <name>`

Revoke a deployment key. Must be the vault owner or the user who created the key.

```bash
vaultenv deploy-key revoke ci-staging
```

This removes the key from the vault config and **rotates the symmetric keys** for all affected environments. The old token becomes permanently useless -- even if someone still has it, it can't decrypt anything.

---

### `vaultenv status`

Show the current state of vaultenv for this project.

```bash
vaultenv status
```

Displays: login status, encryption key status, link status, vault details, your role (owner/approved/pending), available environments, and counts of approved users, pending requests, and deployment keys.

## CI/CD Integration

### Setup

1. Create a deployment key scoped to the environments your pipeline needs:

```bash
vaultenv deploy-key create github-actions-staging --environments staging
```

2. Store the outputted token as a secret in your CI/CD platform:
   - **GitHub Actions**: Settings > Secrets > `VAULTENV_DEPLOY_KEY`
   - **GitLab CI**: Settings > CI/CD > Variables > `VAULTENV_DEPLOY_KEY`
   - **Other**: Whatever secret management your CI platform provides

3. Also create a GitHub personal access token (or fine-grained token) with read access to the `<org>/vaultenv-secrets` repo, and store it as `VAULTENV_GITHUB_TOKEN`.

### GitHub Actions Example

```yaml
jobs:
  deploy:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Install vaultenv
        run: curl -fsSL https://raw.githubusercontent.com/Barestack-io/vaultenv/main/install.sh | sh -s -- --user

      - name: Pull staging secrets
        env:
          VAULTENV_DEPLOY_KEY: ${{ secrets.VAULTENV_DEPLOY_KEY }}
          VAULTENV_GITHUB_TOKEN: ${{ secrets.VAULT_PAT }}
        run: vaultenv pull staging
        # .env.staging is now available for your app

      # Alternative: inject directly into the GitHub Actions environment
      - name: Inject secrets into environment
        env:
          VAULTENV_DEPLOY_KEY: ${{ secrets.VAULTENV_DEPLOY_KEY }}
          VAULTENV_GITHUB_TOKEN: ${{ secrets.VAULT_PAT }}
        run: vaultenv pull staging --format github-env
        # All variables from .env.staging are now available as ${{ env.VAR_NAME }}
```

### Environment Variables

| Variable                | Description                                                             |
| ----------------------- | ----------------------------------------------------------------------- |
| `VAULTENV_DEPLOY_KEY`   | Deployment key token for non-interactive decryption                     |
| `VAULTENV_GITHUB_TOKEN` | GitHub token for vault repo access (takes priority over `GITHUB_TOKEN`) |
| `GITHUB_TOKEN`          | Fallback GitHub token if `VAULTENV_GITHUB_TOKEN` is not set             |
| `VAULTENV_CONFIG_DIR`   | Override the config directory (default: `~/.config/vaultenv`)           |

## Local File Layout

```
~/.config/vaultenv/                    # Global config (all managed by vaultenv)
  config.json                          # GitHub access token + username
  keys/
    private.key                        # X25519 private key (mode 0600)
    public.key                         # X25519 public key

<project>/.vaultenv                    # Per-project link config (gitignored)
```

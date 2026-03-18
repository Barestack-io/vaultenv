# vaultenv

Securely share and store `.env` files with your team using GitHub authentication and client-side encryption.

## How It Works

- **Authentication**: GitHub device flow using the public GitHub CLI client ID (no secrets embedded in the binary)
- **Encryption**: Hybrid NaCl envelope encryption (X25519 + XSalsa20-Poly1305). Each env file is encrypted with a random symmetric key, which is then wrapped per-recipient using their public key.
- **Storage**: Encrypted files are stored in a dedicated private GitHub repo per org (`<org>/vaultenv-secrets`)
- **Key Portability**: Your private key is encrypted with your vault passphrase (Argon2id KDF) and stored in your personal vault repo. Log in on any machine and enter your passphrase to access your keys.

## Install

```bash
go install github.com/scaler/vaultenv/cmd/vaultenv@latest
```

Or build from source:

```bash
git clone https://github.com/scaler/vaultenv.git
cd vaultenv
go build -o vaultenv ./cmd/vaultenv/
```

## Quick Start

```bash
# 1. Authenticate with GitHub
vaultenv login

# 2. Link your project (run inside a git repo)
cd /path/to/your/project
vaultenv link

# 3. Push a shared environment
vaultenv push staging       # pushes .env.staging

# 4. Pull a shared environment
vaultenv pull staging       # downloads and decrypts .env.staging

# 5. Push/pull personal .env
vaultenv push               # pushes .env (only you can decrypt)
vaultenv pull               # pulls your personal .env
```

## Commands

| Command | Description |
|---------|-------------|
| `vaultenv login` | Authenticate with GitHub via device flow |
| `vaultenv init <org>` | Pre-create a vault repo for an org (admin) |
| `vaultenv link` | Link current git repo to vaultenv |
| `vaultenv push [env]` | Encrypt and upload a .env file |
| `vaultenv pull [env]` | Download and decrypt a .env file |
| `vaultenv authorize` | Approve pending access requests (owner) |
| `vaultenv deploy-key create <name>` | Create a CI/CD deployment key |
| `vaultenv deploy-key list` | List deployment keys |
| `vaultenv deploy-key revoke <name>` | Revoke a deployment key |
| `vaultenv status` | Show link status and vault info |

## CI/CD Integration

Create a deployment key scoped to specific environments:

```bash
vaultenv deploy-key create github-actions-staging --environments staging
```

This outputs a token. Store it as `VAULTENV_DEPLOY_KEY` in your CI secrets. You also need a GitHub token with access to the vault repo (`GITHUB_TOKEN` or `VAULTENV_GITHUB_TOKEN`).

### GitHub Actions Example

```yaml
- name: Pull staging env
  env:
    VAULTENV_DEPLOY_KEY: ${{ secrets.VAULTENV_DEPLOY_KEY }}
    VAULTENV_GITHUB_TOKEN: ${{ secrets.VAULT_PAT }}
  run: vaultenv pull staging

# Or inject directly into the environment:
- name: Inject env vars
  env:
    VAULTENV_DEPLOY_KEY: ${{ secrets.VAULTENV_DEPLOY_KEY }}
    VAULTENV_GITHUB_TOKEN: ${{ secrets.VAULT_PAT }}
  run: eval $(vaultenv pull staging --export)
```

Output modes:
- `vaultenv pull staging` — writes `.env.staging` file
- `vaultenv pull staging --export` — prints `export KEY=VALUE` lines for shell eval
- `vaultenv pull staging --format github-env` — appends to `$GITHUB_ENV`

## Security

- **Client-side encryption**: All encryption/decryption happens on your machine. GitHub never sees plaintext secrets.
- **Envelope encryption**: Random symmetric key per file, sealed per-recipient with NaCl `box`. Even the vault owner can't decrypt personal environments.
- **Passphrase-protected keys**: Private keys stored in GitHub are encrypted with Argon2id (memory-hard KDF).
- **Scoped deployment keys**: CI/CD keys only decrypt specified environments. Revocation triggers key rotation.
- **No embedded secrets**: The device flow uses GitHub CLI's public client ID — no client secret needed.

## Architecture

```
~/.config/vaultenv/           # Local state
  config.json                 # GitHub token + username
  keys/private.key            # X25519 private key
  keys/public.key             # X25519 public key

<user>/vaultenv-secrets       # Personal vault (GitHub repo)
  keys/<user>.key.enc         # Passphrase-encrypted private key
  keys/<user>.pub             # Public key

<org>/vaultenv-secrets        # Org vault (GitHub repo)
  <repo>/vault.json           # Access control + environment list
  <repo>/environments/
    shared/<env>.enc          # Encrypted env file
    shared/<env>.json         # Key envelopes per recipient
    personal/<user>.enc       # Personal encrypted env
    personal/<user>.json      # Personal key envelope
```

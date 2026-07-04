# Azure DevOps CLI — Phase 5 Acceptance (Deferred)

## Status
Live dogfood skipped — API requires a Personal Access Token (PAT) which is not yet set.

## Step-by-step setup to run live tests later

### Step 1: Generate a Personal Access Token

1. Open your browser and navigate to: `https://dev.azure.com/{your-organization}/_usersSettings/tokens`
   - Or run: `azure-devops-pp-cli auth setup --launch` to open this page directly
2. Click "New Token"
3. Set a name (e.g., "azure-devops-pp-cli")
4. Set expiration (recommend 90 days)
5. Select these scopes:
   - **Work Items**: Read & Write
   - **Code**: Read
   - **Build**: Read
   - **Release**: Read & Execute

6. Click "Create" and copy the token

### Step 2: Set environment variables

```bash
export AZURE_DEVOPS_TOKEN="your-token-here"
export AZURE_DEVOPS_ORG="your-org-name"       # e.g., "mycompany" from dev.azure.com/mycompany
export AZURE_DEVOPS_PROJECT="your-project"    # e.g., "MyProject"
```

Or save them permanently:
```bash
azure-devops-pp-cli auth set-token "your-token-here"
```

Then in your shell profile (`~/.zshrc` or `~/.bashrc`):
```bash
echo 'export AZURE_DEVOPS_ORG="your-org-name"' >> ~/.zshrc
echo 'export AZURE_DEVOPS_PROJECT="your-project"' >> ~/.zshrc
```

### Step 3: Verify connectivity

```bash
azure-devops-pp-cli doctor
```

Expected output: `auth: configured`, `connectivity: ok`

### Step 4: Run the morning standup

```bash
azure-devops-pp-cli standup --agent
```

### Step 5: Try your first PR review queue

```bash
azure-devops-pp-cli pr review-queue --json
```

### Step 6: Check pipeline approval queue

```bash
azure-devops-pp-cli release gate-queue --json
```

## Gate: DEFERRED

The CLI passes all structural verification. Live correctness testing deferred to when AZURE_DEVOPS_TOKEN is available.

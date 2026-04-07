# AWS Setup Guide — JobHuntr

This guide walks through deploying JobHuntr on AWS from scratch. The app is
currently wired for Render.com (see `render.yaml`), but everything it needs
maps cleanly to AWS primitives.

---

## Architecture Overview

```
Internet
    │
    ▼
[Route 53]  ──── optional, for custom domain
    │
    ▼
[ACM Certificate]  ──── HTTPS termination
    │
    ▼
[Application Load Balancer]  (port 443 → 8080)
    │
    ▼
[ECS Fargate Task]  ──── your Docker container
    │          │
    │          └── [EFS Mount]  ──── ./output/ (PDFs, markdown files)
    │
    ▼
[RDS PostgreSQL]  ──── primary database
    │
    ▼
[Secrets Manager]  ──── all env vars / secrets
```

**Why these services:**

| AWS Service | Replaces | Why |
|-------------|----------|-----|
| ECR | Docker Hub | Private image registry in your AWS account |
| ECS Fargate | Render web service | Runs your Docker container, no servers to manage |
| RDS (PostgreSQL 16) | Render Postgres | Managed Postgres |
| EFS | Local filesystem | Persists `./output/` across container restarts |
| Secrets Manager | Render env vars | Stores all secrets; injects into ECS at runtime |
| ALB | Render's edge proxy | HTTPS termination, health checks, routing |
| ACM | Render's TLS | Free managed SSL cert |

---

## Prerequisites

- AWS account with billing set up
- AWS CLI installed: `brew install awscli` or follow https://docs.aws.amazon.com/cli/latest/userguide/install-cliv2.html
- Docker installed and running locally
- A domain name (optional, but needed for HTTPS with a real cert)

**Configure the CLI:**
```
aws configure
```
Enter your Access Key ID, Secret Access Key, region (e.g. `us-east-1`), and output format (`json`).

Verify it works:
```
aws sts get-caller-identity
```

---

## Step 1 — IAM: Create a Deployment User

Don't use your root account for deployments. Create a dedicated IAM user.

1. Go to **IAM → Users → Create user**
2. Name: `jobhuntr-deploy`
3. Attach these managed policies directly:
   - `AmazonECS_FullAccess`
   - `AmazonEC2ContainerRegistryFullAccess`
   - `AmazonRDSFullAccess`
   - `AmazonEFSFullAccess`
   - `SecretsManagerReadWrite`
   - `ElasticLoadBalancingFullAccess`
   - `AmazonVPCFullAccess`
   - `IAMFullAccess` (needed to create the ECS task execution role)
4. Create access keys for this user and run `aws configure` with them.

> **Note:** In production you'd scope these down to least-privilege. For initial
> setup, broad permissions let you iterate without hitting walls.

---

## Step 2 — VPC and Networking

You can use the **default VPC** that AWS creates in every region. This is the
simplest path for a first deployment. Skip to Step 3 if you want to use the
default.

**To confirm your default VPC exists:**
```
aws ec2 describe-vpcs --filters "Name=isDefault,Values=true" --query "Vpcs[0].VpcId"
```

Note the VPC ID (e.g. `vpc-0abc1234`). You'll need it later.

**Get your subnet IDs** (you need at least 2 for the ALB):
```
aws ec2 describe-subnets --filters "Name=defaultForAz,Values=true" --query "Subnets[*].SubnetId"
```

Note at least 2 subnet IDs from different availability zones.

### Security Groups

You need two security groups:

**ALB Security Group** — allows public HTTPS traffic in:
```
aws ec2 create-security-group --group-name jobhuntr-alb-sg --description "JobHuntr ALB" --vpc-id <your-vpc-id>
```
Then allow inbound HTTP and HTTPS:
```
aws ec2 authorize-security-group-ingress --group-id <alb-sg-id> --protocol tcp --port 80 --cidr 0.0.0.0/0
aws ec2 authorize-security-group-ingress --group-id <alb-sg-id> --protocol tcp --port 443 --cidr 0.0.0.0/0
```

**App Security Group** — allows traffic from ALB to the container on port 8080:
```
aws ec2 create-security-group --group-name jobhuntr-app-sg --description "JobHuntr App" --vpc-id <your-vpc-id>
```
Then allow the ALB to reach the app:
```
aws ec2 authorize-security-group-ingress --group-id <app-sg-id> --protocol tcp --port 8080 --source-group <alb-sg-id>
```

**RDS Security Group** — allows the app to reach Postgres on port 5432:
```
aws ec2 create-security-group --group-name jobhuntr-rds-sg --description "JobHuntr RDS" --vpc-id <your-vpc-id>
aws ec2 authorize-security-group-ingress --group-id <rds-sg-id> --protocol tcp --port 5432 --source-group <app-sg-id>
```

Save all three security group IDs — you'll use them throughout this guide.

---

## Step 3 — ECR: Push Your Docker Image

ECR is AWS's private Docker registry. You push your image here; ECS pulls from it.

**Create the repository:**
```
aws ecr create-repository --repository-name jobhuntr --region us-east-1
```

Note the `repositoryUri` from the output. It looks like:
`123456789012.dkr.ecr.us-east-1.amazonaws.com/jobhuntr`

**Authenticate Docker to ECR:**
```
aws ecr get-login-password --region us-east-1 | docker login --username AWS --password-stdin 123456789012.dkr.ecr.us-east-1.amazonaws.com
```

**Build and push the image:**
```
docker build -t jobhuntr .
docker tag jobhuntr:latest 123456789012.dkr.ecr.us-east-1.amazonaws.com/jobhuntr:latest
docker push 123456789012.dkr.ecr.us-east-1.amazonaws.com/jobhuntr:latest
```

> The production `Dockerfile` at the repo root is a two-stage build. It installs
> Chromium for PDF generation. The final image runs as a non-root user on port 8080.

---

## Step 4 — RDS: Create the PostgreSQL Database

**Create a DB subnet group** (RDS needs to know which subnets it can use):

1. Go to **RDS → Subnet groups → Create DB subnet group**
2. Name: `jobhuntr-db-subnet-group`
3. VPC: your VPC
4. Add all subnets across at least 2 AZs

**Create the database:**

1. Go to **RDS → Create database**
2. Engine: **PostgreSQL**
3. Version: **16.x** (match what Docker Compose uses)
4. Template: **Free tier** (for dev) or **Production** (for prod)
5. DB instance identifier: `jobhuntr-db`
6. Master username: `jobhuntr`
7. Master password: generate a strong password and save it — you'll put it in Secrets Manager
8. Instance class: `db.t3.micro` (free tier eligible) or `db.t3.small` for production
9. Storage: 20 GB gp2 (can autoscale)
10. VPC: your VPC
11. VPC security group: select `jobhuntr-rds-sg` (the one you created above)
12. **Publicly accessible: No** — the app reaches it via VPC, not the internet
13. Initial database name: `jobhuntr`
14. Enable automated backups: yes, 7-day retention

After creation, note the **Endpoint** — it looks like:
`jobhuntr-db.xxxxxxxxxxxx.us-east-1.rds.amazonaws.com`

Your `DATABASE_URL` will be:
```
postgres://jobhuntr:<password>@jobhuntr-db.xxxxxxxxxxxx.us-east-1.rds.amazonaws.com:5432/jobhuntr
```

> **Migrations:** The app runs database migrations automatically on startup via
> `store.Open()` in `internal/store/migrate.go`. You don't need to run them
> manually — just make sure `DATABASE_URL` is correct and the app can reach RDS.

---

## Step 5 — Secrets Manager: Store All Secrets

Secrets Manager is how ECS gets your environment variables at runtime. You store
all secrets here and reference them in the ECS task definition.

Go to **Secrets Manager → Store a new secret**.

For each secret below, choose **Other type of secret** and add it as a
plaintext string (not key/value — the app reads individual env vars).

Actually, the easiest approach is to store them all as a **single JSON secret**:

1. Choose **Other type of secret**
2. Choose **Key/value pairs**
3. Add all your secrets as key/value pairs:

| Key | Value |
|-----|-------|
| `SESSION_SECRET` | output of: `openssl rand -hex 32` |
| `DATABASE_URL` | `postgres://jobhuntr:<password>@<rds-endpoint>:5432/jobhuntr` |
| `GITHUB_CLIENT_ID` | from GitHub OAuth App |
| `GITHUB_CLIENT_SECRET` | from GitHub OAuth App |
| `GOOGLE_CLIENT_ID` | from Google Cloud Console (optional) |
| `GOOGLE_CLIENT_SECRET` | from Google Cloud Console (optional) |
| `SERPAPI_KEY` | from serpapi.com (optional) |
| `ANTHROPIC_API_KEY` | from console.anthropic.com (optional) |
| `ADMIN_PASSWORD` | strong password for /admin panel |
| `GOOGLE_DRIVE_CLIENT_ID` | from Google Cloud Console (optional) |
| `GOOGLE_DRIVE_CLIENT_SECRET` | from Google Cloud Console (optional) |

4. Secret name: `jobhuntr/production`
5. Leave rotation disabled for now

Note the **Secret ARN** — you'll need it for the ECS task execution role policy.

> **SESSION_SECRET is critical:** This must stay constant across deployments.
> If it changes, all active user sessions are invalidated. Never regenerate it
> unless you intend to log everyone out.

---

## Step 6 — OAuth: Update Callback URLs

Before deploying, update your OAuth apps with the new callback URLs. You need
your domain name for this — use a placeholder like `https://your-alb-dns-name`
if you don't have a custom domain yet.

**GitHub OAuth App** (https://github.com/settings/developers):
- Homepage URL: `https://yourdomain.com`
- Callback URL: `https://yourdomain.com/auth/github/callback`

**Google OAuth App** (https://console.cloud.google.com → APIs & Services → Credentials):
- Authorized redirect URIs: `https://yourdomain.com/auth/google/callback`

**Google Drive OAuth** (same Google Cloud project):
- Authorized redirect URIs: `https://yourdomain.com/auth/google-drive/callback`

> The `base_url` in `config.yaml` must match the URL your users actually hit.
> It's used to construct redirect URLs for OAuth callbacks. It's set via the
> config file, not an env var — you'll mount the config file into the container
> (see Step 8).

---

## Step 7 — EFS: Persistent Storage for Output Files

The app writes generated PDFs, Markdown, and DOCX files to `./output/` inside
the container. On ECS Fargate, the container filesystem is ephemeral — files
disappear when the container restarts. EFS gives you a persistent network
filesystem that survives restarts.

**Create the EFS filesystem:**

1. Go to **EFS → Create file system**
2. Name: `jobhuntr-output`
3. VPC: your VPC
4. Click **Customize** to set security groups:
   - Mount targets: add one per AZ, using `jobhuntr-app-sg`

Note the **File system ID** (e.g. `fs-0abc1234`).

**Allow the app security group to reach EFS on port 2049:**
```
aws ec2 authorize-security-group-ingress --group-id <app-sg-id> --protocol tcp --port 2049 --source-group <app-sg-id>
```

You'll reference the EFS file system ID in the ECS task definition below.

---

## Step 8 — IAM: ECS Task Execution Role

ECS needs an IAM role to pull images from ECR, read secrets from Secrets Manager,
and write logs to CloudWatch.

**Create the role:**

1. Go to **IAM → Roles → Create role**
2. Trusted entity: **AWS service → Elastic Container Service → Elastic Container Service Task**
3. Attach these policies:
   - `AmazonECSTaskExecutionRolePolicy` (allows ECR pull + CloudWatch logs)
4. Role name: `jobhuntr-ecs-execution-role`

**Add a custom inline policy for Secrets Manager access:**

After creating the role, add an inline policy:
```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "secretsmanager:GetSecretValue"
      ],
      "Resource": "arn:aws:secretsmanager:us-east-1:123456789012:secret:jobhuntr/production-*"
    }
  ]
}
```

Replace the ARN with your actual secret ARN (from Step 5) plus a wildcard `*`
to cover the version suffix AWS appends.

Note the **Role ARN** — it looks like:
`arn:aws:iam::123456789012:role/jobhuntr-ecs-execution-role`

---

## Step 9 — ECS: Cluster, Task Definition, and Service

### Create a CloudWatch Log Group

First, create a log group so container logs have somewhere to go:
```
aws logs create-log-group --log-group-name /ecs/jobhuntr --region us-east-1
```

### Create the ECS Cluster

1. Go to **ECS → Clusters → Create cluster**
2. Cluster name: `jobhuntr`
3. Infrastructure: **AWS Fargate (serverless)** — no EC2 instances to manage
4. Create

### Create the Task Definition

1. Go to **ECS → Task definitions → Create new task definition**
2. Task definition family: `jobhuntr`
3. Launch type: **Fargate**
4. Task execution role: `jobhuntr-ecs-execution-role` (from Step 8)
5. CPU: `1 vCPU`
6. Memory: `2 GB` (Chromium for PDF generation is memory-hungry; 1 GB will OOM)

**Container definition:**

- Name: `jobhuntr`
- Image URI: `123456789012.dkr.ecr.us-east-1.amazonaws.com/jobhuntr:latest`
- Essential: yes
- Port mappings: `8080` TCP

**Environment variables — from Secrets Manager:**

In the container's environment variable section, add each secret as a
`valueFrom` reference (not a hardcoded value). For each key in your
`jobhuntr/production` secret:

| Name | Value source | Secret key |
|------|-------------|------------|
| `SESSION_SECRET` | Secrets Manager ARN | `SESSION_SECRET` |
| `DATABASE_URL` | Secrets Manager ARN | `DATABASE_URL` |
| `GITHUB_CLIENT_ID` | Secrets Manager ARN | `GITHUB_CLIENT_ID` |
| `GITHUB_CLIENT_SECRET` | Secrets Manager ARN | `GITHUB_CLIENT_SECRET` |
| `SERPAPI_KEY` | Secrets Manager ARN | `SERPAPI_KEY` |
| `ANTHROPIC_API_KEY` | Secrets Manager ARN | `ANTHROPIC_API_KEY` |
| *(add others as needed)* | | |

In the task definition JSON, this looks like:
```json
"secrets": [
  {
    "name": "SESSION_SECRET",
    "valueFrom": "arn:aws:secretsmanager:us-east-1:123456789012:secret:jobhuntr/production:SESSION_SECRET::"
  },
  {
    "name": "DATABASE_URL",
    "valueFrom": "arn:aws:secretsmanager:us-east-1:123456789012:secret:jobhuntr/production:DATABASE_URL::"
  }
]
```

The format is: `<secret-arn>:<json-key>::`

**config.yaml — mount as a volume:**

The app reads `config.yaml` at startup (see `internal/config/config.go`). You
need to get this file into the container. Two options:

**Option A (simpler):** Bake it into the Docker image by adding it before the
`COPY` step in your Dockerfile. Update `config.yaml` with production values
(using `${ENV_VAR}` substitutions — the app will expand them).

**Option B (flexible):** Store `config.yaml` content in S3 and use an
init container or entrypoint script to download it before the app starts.

For initial setup, Option A is fine. The config file contains no hardcoded
secrets (they all use `${VAR}` substitution), so it's safe to bake in.

**EFS volume (for output directory):**

In the task definition, add a volume:
- Volume name: `jobhuntr-output`
- EFS file system ID: `fs-0abc1234` (from Step 7)
- Root directory: `/`

In the container definition, add a mount point:
- Source volume: `jobhuntr-output`
- Container path: `/app/output`

**Health check:**

The container health check should hit the `/healthz` endpoint:
```
CMD-SHELL, curl -f http://localhost:8080/healthz || exit 1
```

**Logging:**

Configure the `awslogs` log driver:
- Log driver: `awslogs`
- `awslogs-group`: `/ecs/jobhuntr`
- `awslogs-region`: `us-east-1`
- `awslogs-stream-prefix`: `ecs`

### Create the ECS Service

1. Go to your `jobhuntr` cluster → **Services → Create**
2. Launch type: **Fargate**
3. Task definition: `jobhuntr` (latest revision)
4. Service name: `jobhuntr`
5. Desired tasks: `1` (start with 1; scale up later if needed)
6. VPC: your VPC
7. Subnets: the 2+ subnets you noted in Step 2
8. Security groups: `jobhuntr-app-sg`
9. Public IP: **Disabled** (traffic comes through the ALB, not directly)
10. Load balancing: select **Application Load Balancer** (create below, then come back)

> You may need to create the ALB first (Step 10), then come back and attach it
> to the service, or configure the service without a load balancer first and
> update it after.

---

## Step 10 — ALB: Application Load Balancer

### Create the Load Balancer

1. Go to **EC2 → Load Balancers → Create load balancer**
2. Type: **Application Load Balancer**
3. Name: `jobhuntr-alb`
4. Scheme: **Internet-facing**
5. IP type: IPv4
6. VPC: your VPC
7. Subnets: select at least 2 subnets (from different AZs)
8. Security groups: `jobhuntr-alb-sg`

### Create the Target Group

1. Go to **EC2 → Target Groups → Create target group**
2. Target type: **IP addresses** (required for Fargate)
3. Protocol: HTTP
4. Port: `8080`
5. VPC: your VPC
6. Health check path: `/healthz`
7. Health check interval: 30s
8. Healthy threshold: 2
9. Unhealthy threshold: 3
10. Name: `jobhuntr-tg`

You don't register targets manually — ECS registers them automatically.

### Configure ALB Listeners

**HTTP listener (port 80):**
- Action: Redirect to HTTPS (port 443, 301 permanent)

**HTTPS listener (port 443):**
- Default action: Forward to `jobhuntr-tg`
- SSL certificate: from ACM (see Step 11)

> Add the HTTPS listener after your ACM certificate is issued (Step 11).

---

## Step 11 — ACM: SSL Certificate

1. Go to **ACM (Certificate Manager) → Request a certificate**
2. Certificate type: **Public**
3. Domain name: `yourdomain.com` and `*.yourdomain.com` (add both)
4. Validation method: **DNS validation** (recommended)
5. Request

ACM will give you CNAME records to add to your DNS. If you're using Route 53,
there's a one-click button to add them. If you're using another DNS provider
(Cloudflare, Namecheap, etc.), manually add the CNAME records.

Once DNS propagates (usually 5–30 minutes), the certificate status changes to
**Issued**.

Go back to your ALB's HTTPS listener and attach this certificate.

---

## Step 12 — Route 53: Custom Domain (Optional)

If your domain is managed elsewhere (Cloudflare, etc.), skip this and just add
a CNAME from your domain to the ALB's DNS name.

If you want to use Route 53:

1. **Transfer or create a hosted zone:**
   - **Route 53 → Hosted zones → Create hosted zone**
   - Domain name: `yourdomain.com`
   - Type: Public hosted zone

2. AWS gives you 4 NS records — update your domain registrar to use them.

3. **Create an A record pointing to the ALB:**
   - Record name: `yourdomain.com` (apex) or `app.yourdomain.com`
   - Record type: **A**
   - Alias: **Yes**
   - Route traffic to: **Alias to Application and Classic Load Balancer**
   - Select your region and your `jobhuntr-alb`

---

## Step 13 — Final Checklist Before First Deploy

Before starting the ECS service for the first time, verify:

- [ ] RDS instance is in `Available` state
- [ ] RDS security group allows port 5432 from `jobhuntr-app-sg`
- [ ] EFS mount targets are in `Available` state
- [ ] All secrets are in Secrets Manager (`jobhuntr/production`)
- [ ] ECS task execution role can read the Secrets Manager secret
- [ ] ECR has the latest image pushed
- [ ] `config.yaml` has correct `base_url` (your production domain)
- [ ] OAuth callback URLs updated in GitHub and Google consoles
- [ ] ACM certificate is in `Issued` state
- [ ] ALB HTTPS listener has the ACM certificate attached
- [ ] Target group health check path is `/healthz`

**Start the service** (or set desired count to 1 if you created it stopped).

**Watch the logs:**
```
aws logs tail /ecs/jobhuntr --follow --region us-east-1
```

On a healthy first startup you should see:
- Database migration logs (14 migrations applied)
- "Scheduler started" message
- "Listening on :8080" or similar

**Check the target group health:**
```
aws elbv2 describe-target-health --target-group-arn <tg-arn>
```

Targets should move from `initial` → `healthy` within 1–2 minutes.

---

## Step 14 — Deploying Updates

Every time you change the code, repeat this to deploy:

```
docker build -t jobhuntr .
docker tag jobhuntr:latest 123456789012.dkr.ecr.us-east-1.amazonaws.com/jobhuntr:latest
aws ecr get-login-password --region us-east-1 | docker login --username AWS --password-stdin 123456789012.dkr.ecr.us-east-1.amazonaws.com
docker push 123456789012.dkr.ecr.us-east-1.amazonaws.com/jobhuntr:latest
aws ecs update-service --cluster jobhuntr --service jobhuntr --force-new-deployment --region us-east-1
```

ECS pulls the new image, starts a new task, waits for it to pass health checks,
then drains and stops the old task. Zero-downtime rolling deploy.

---

## Step 15 — Cost Estimate

Rough monthly costs for a low-traffic personal deployment in `us-east-1`:

| Service | Config | ~Monthly Cost |
|---------|--------|--------------|
| ECS Fargate | 1 vCPU, 2 GB, 24/7 | ~$35 |
| RDS PostgreSQL | db.t3.micro, 20 GB | ~$15 |
| ALB | 1 LCU baseline | ~$18 |
| EFS | <1 GB storage | <$1 |
| ECR | <1 GB storage | <$1 |
| CloudWatch Logs | Low volume | <$1 |
| ACM Certificate | Public cert | Free |
| Data transfer | Low volume | ~$1 |
| **Total** | | **~$70/month** |

**To cut costs:**
- Stop the ECS service when not in use (scale desired count to 0)
- Use RDS `db.t3.micro` and enable stop/start schedule
- Consider running on EC2 `t3.small` with the container instead of Fargate
  (~$15/mo vs $35/mo for Fargate) — use the existing `deploy/jobhuntr.service`
  systemd unit as reference

---

## Appendix: Environment Variables Reference

All variables the app reads, in one place:

| Variable | Required | Where to get it |
|----------|----------|-----------------|
| `SESSION_SECRET` | Yes | `openssl rand -hex 32` |
| `DATABASE_URL` | Yes | RDS endpoint (Step 4) |
| `GITHUB_CLIENT_ID` | Yes | https://github.com/settings/developers |
| `GITHUB_CLIENT_SECRET` | Yes | Same |
| `GOOGLE_CLIENT_ID` | No | https://console.cloud.google.com |
| `GOOGLE_CLIENT_SECRET` | No | Same |
| `SERPAPI_KEY` | No | https://serpapi.com/manage-api-key |
| `JSEARCH_KEY` | No | https://rapidapi.com/letscrape-6bfp5hPzrr6/api/jsearch |
| `ANTHROPIC_API_KEY` | No | https://console.anthropic.com/settings/keys |
| `ADMIN_PASSWORD` | No | Any strong password |
| `GOOGLE_DRIVE_CLIENT_ID` | No | Google Cloud Console |
| `GOOGLE_DRIVE_CLIENT_SECRET` | No | Same |

Variables are referenced in `config.yaml` using `${VAR_NAME}` syntax. The
config loader in `internal/config/config.go` expands them at startup.

---

## Appendix: Useful AWS CLI Commands

```
# View running ECS tasks
aws ecs list-tasks --cluster jobhuntr --region us-east-1

# Stream logs live
aws logs tail /ecs/jobhuntr --follow --region us-east-1

# Force a new deployment (after pushing a new image)
aws ecs update-service --cluster jobhuntr --service jobhuntr --force-new-deployment --region us-east-1

# Scale the service down (stop paying for compute)
aws ecs update-service --cluster jobhuntr --service jobhuntr --desired-count 0 --region us-east-1

# Scale back up
aws ecs update-service --cluster jobhuntr --service jobhuntr --desired-count 1 --region us-east-1

# Check target group health
aws elbv2 describe-target-health --target-group-arn <arn> --region us-east-1

# Get RDS instance status
aws rds describe-db-instances --db-instance-identifier jobhuntr-db --query "DBInstances[0].DBInstanceStatus"

# Describe a Secrets Manager secret
aws secretsmanager get-secret-value --secret-id jobhuntr/production --region us-east-1
```

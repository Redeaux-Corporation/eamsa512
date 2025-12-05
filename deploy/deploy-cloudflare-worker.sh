#!/bin/bash
# EAMSA 512 - Cloudflare Workers Deployment
# Deploy EAMSA 512 as a Cloudflare Worker

set -euo pipefail

echo "╔════════════════════════════════════════════════════════╗"
echo "║  EAMSA 512 Cloudflare Workers Deployment v1.0.0       ║"
echo "╚════════════════════════════════════════════════════════╝"
echo ""

# Check if wrangler is installed
if ! command -v wrangler &> /dev/null; then
    echo "[INFO] Installing Wrangler CLI..."
    npm install -g wrangler
fi

# Check if logged in
if ! wrangler whoami &>/dev/null; then
    echo "[INFO] Please log in to Cloudflare"
    wrangler login
fi

# Create wrangler.toml
cat > wrangler.toml <<'EOF'
name = "eamsa512"
type = "javascript"
account_id = ""
workers_dev = true
route = ""
zone_id = ""

[env.production]
name = "eamsa512-prod"
route = "api.eamsa512.com/*"

[env.staging]
name = "eamsa512-staging"
route = "staging-api.eamsa512.com/*"

[[env.production.kv_namespaces]]
binding = "CACHE"
id = ""
preview_id = ""

[[env.production.r2_buckets]]
binding = "LOGS"
bucket_name = "eamsa512-logs"

[build]
command = "npm install && npm run build"
cwd = "./"

[[migrations]]
tag = "v1"
new_classes = ["EncryptionWorker"]
EOF

# Create package.json
cat > package.json <<'EOF'
{
  "name": "eamsa512-worker",
  "version": "1.0.0",
  "type": "module",
  "scripts": {
    "build": "tsc",
    "dev": "wrangler dev",
    "deploy": "wrangler deploy",
    "deploy:staging": "wrangler deploy --env staging"
  },
  "dependencies": {
    "wrangler": "^3.0.0"
  },
  "devDependencies": {
    "typescript": "^5.0.0",
    "@types/node": "^20.0.0"
  }
}
EOF

# Create main worker file
mkdir -p src
cat > src/index.ts <<'EOF'
import { Router } from 'itty-router'

interface EncryptRequest {
  plaintext: string
  master_key: string
  nonce?: string
}

interface EncryptResponse {
  ciphertext: string
  nonce: string
  tag: string
  timestamp: string
  size: number
}

const router = Router()

// Encryption endpoint
router.post('/api/v1/encrypt', async (request: Request, env: any) => {
  try {
    const body: EncryptRequest = await request.json()

    // Validate input
    if (!body.plaintext || !body.master_key) {
      return new Response(
        JSON.stringify({ error: 'Missing required fields' }),
        { status: 400, headers: { 'Content-Type': 'application/json' } }
      )
    }

    // Cache key for KV storage
    const cacheKey = `encrypt:${body.master_key.substring(0, 8)}`

    // Try to get from cache
    let result = await env.CACHE.get(cacheKey)

    if (!result) {
      // Call backend service
      const backendUrl = 'https://backend.eamsa512.com/api/v1/encrypt'
      const response = await fetch(backendUrl, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(body)
      })

      result = await response.json()

      // Cache for 5 minutes
      await env.CACHE.put(cacheKey, JSON.stringify(result), {
        expirationTtl: 300
      })
    }

    return new Response(JSON.stringify(result), {
      headers: { 'Content-Type': 'application/json' }
    })
  } catch (error) {
    return new Response(
      JSON.stringify({ error: String(error) }),
      { status: 500, headers: { 'Content-Type': 'application/json' } }
    )
  }
})

// Decryption endpoint
router.post('/api/v1/decrypt', async (request: Request, env: any) => {
  try {
    const body = await request.json()

    // Validate input
    if (!body.ciphertext || !body.master_key) {
      return new Response(
        JSON.stringify({ error: 'Missing required fields' }),
        { status: 400, headers: { 'Content-Type': 'application/json' } }
      )
    }

    // Call backend service
    const backendUrl = 'https://backend.eamsa512.com/api/v1/decrypt'
    const response = await fetch(backendUrl, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body)
    })

    const result = await response.json()

    return new Response(JSON.stringify(result), {
      headers: { 'Content-Type': 'application/json' }
    })
  } catch (error) {
    return new Response(
      JSON.stringify({ error: String(error) }),
      { status: 500, headers: { 'Content-Type': 'application/json' } }
    )
  }
})

// Health check
router.get('/api/v1/health', async (request: Request) => {
  return new Response(
    JSON.stringify({
      status: 'ok',
      version: '1.0.0',
      timestamp: new Date().toISOString()
    }),
    { headers: { 'Content-Type': 'application/json' } }
  )
})

// Rate limiting middleware
function rateLimit(request: Request) {
  const ip = request.headers.get('cf-connecting-ip') || 'unknown'
  // Implement rate limiting logic
  return true
}

// Main handler
export default {
  fetch: router.handle,
  async scheduled(event: ScheduledEvent, env: any) {
    // Run maintenance tasks
    // - Rotate keys
    // - Cleanup old logs
    // - Sync with backend
  }
}
EOF

# Create TypeScript config
cat > tsconfig.json <<'EOF'
{
  "compilerOptions": {
    "target": "ES2022",
    "module": "ES2022",
    "lib": ["ES2022"],
    "declaration": true,
    "outDir": "dist",
    "strict": true,
    "esModuleInterop": true,
    "skipLibCheck": true,
    "forceConsistentCasingInFileNames": true,
    "resolveJsonModule": true
  },
  "include": ["src/**/*"],
  "exclude": ["node_modules"]
}
EOF

echo "[INFO] Building TypeScript..."
npm install
npm run build

echo "[INFO] Deploying to Cloudflare..."
wrangler deploy

echo ""
echo "╔════════════════════════════════════════════════════════╗"
echo "║         Deployment Complete!                          ║"
echo "╚════════════════════════════════════════════════════════╝"
echo ""
echo "Next steps:"
echo "  1. Update wrangler.toml with your account_id and zone_id"
echo "  2. Create KV namespace: wrangler kv:namespace create CACHE"
echo "  3. Deploy: npm run deploy"
echo "  4. Test: curl https://<your-worker-url>/api/v1/health"
echo ""

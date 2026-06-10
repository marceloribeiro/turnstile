#!/usr/bin/env bash
set -e

ENVIRONMENT="${1:-production}"
CONFIG_FILE="deploy.json"
APP_NAME="turnstile-marketing"

if [ ! -f "$CONFIG_FILE" ]; then
    echo "❌ Configuration file not found: $CONFIG_FILE"
    exit 1
fi

echo "🚀 Deploying Turnstile marketing site to $ENVIRONMENT..."
echo ""

python3 << PYEOF > /tmp/deploy_vars.sh
import json
import sys
import datetime

config_file = "$CONFIG_FILE"
environment = "$ENVIRONMENT"

try:
    with open(config_file, 'r') as f:
        config = json.load(f)

    if environment not in config:
        print(f"❌ Environment '{environment}' not found in config file", file=sys.stderr)
        sys.exit(1)

    env_config = config[environment]
    server = env_config.get('server')
    github = env_config.get('github')
    deploy_location = env_config.get('deploy_location')
    deploy_branch = env_config.get('deploy_branch', 'main')

    timestamp = datetime.datetime.now().strftime('%Y%m%d%H%M%S')
    release_path = f"{deploy_location}/releases/{timestamp}"

    print(f'export DEPLOY_SERVER="{server}"')
    print(f'export DEPLOY_GITHUB="{github}"')
    print(f'export DEPLOY_LOCATION="{deploy_location}"')
    print(f'export DEPLOY_BRANCH="{deploy_branch}"')
    print(f'export DEPLOY_RELEASE_PATH="{release_path}"')
    print(f'export DEPLOY_TIMESTAMP="{timestamp}"')

except Exception as e:
    print(f"❌ Error parsing config: {e}", file=sys.stderr)
    sys.exit(1)
PYEOF

source /tmp/deploy_vars.sh

echo "📋 Deployment Configuration:"
echo "   Server: $DEPLOY_SERVER"
echo "   GitHub: $DEPLOY_GITHUB"
echo "   Deploy Location: $DEPLOY_LOCATION"
echo "   Branch: $DEPLOY_BRANCH"
echo "   Release Path: $DEPLOY_RELEASE_PATH"
echo ""

# Pick a production env file if either form exists. We build with it locally and
# also ship it to the release as `.env.production` so the runtime picks it up.
ENV_PROD_FILE=".env.production.local"
if [ ! -f "$ENV_PROD_FILE" ]; then
    ENV_PROD_FILE=".env.production"
fi
if [ ! -f "$ENV_PROD_FILE" ]; then
    echo "⚠️  No .env.production.local or .env.production found. Build will proceed without one."
    ENV_PROD_FILE=""
fi

# Next.js loads env files in this priority during `next build`:
#   .env.production.local > .env.local > .env.production > .env
# A dev value in .env.local would otherwise clobber .env.production for
# NEXT_PUBLIC_* keys at build time (they get inlined into the bundle).
# Materialize .env.production as .env.production.local for the build so it
# wins, and clean up on exit so the dev workflow isn't affected.
PROD_LOCAL_CREATED=0
cleanup_prod_local() {
    if [ "$PROD_LOCAL_CREATED" = "1" ]; then
        rm -f .env.production.local
    fi
}
trap cleanup_prod_local EXIT
if [ -f ".env.production" ] && [ ! -f ".env.production.local" ]; then
    cp .env.production .env.production.local
    PROD_LOCAL_CREATED=1
    echo "📋 Materialized .env.production.local from .env.production for the build (auto-removed on exit)"
fi

# Build locally — building on the small web host kills it (OOM / 100% CPU).
echo ""
echo "🏗️  Building Next.js application locally..."

if ! command -v node &> /dev/null; then
    echo "❌ Error: Node.js not found. Please install Node.js."
    exit 1
fi

echo "   Using Node.js version: $(node -v)"

if [ ! -d "node_modules" ] || [ "package.json" -nt "node_modules" ]; then
    echo "📦 Installing npm dependencies..."
    npm install
    echo "✅ Dependencies installed"
else
    echo "✅ Dependencies already installed (skipping)"
fi

# `next build` reads .env.production automatically when NODE_ENV=production.
echo "🏗️  Building production bundle..."
if NODE_ENV=production npm run build; then
    echo "✅ Build completed"
else
    echo "❌ Error: Build failed. Please fix build errors before deploying."
    exit 1
fi

# Standalone output is the load-bearing artifact — everything past here assumes it.
if [ ! -f ".next/standalone/server.js" ]; then
    echo "❌ Error: .next/standalone/server.js not found."
    echo "   Make sure next.config.ts sets output: 'standalone'."
    exit 1
fi
echo "✅ Local build completed successfully"
echo ""

echo "🔐 Connecting to server: $DEPLOY_SERVER"

echo "📁 Creating release directory on server..."
ssh "$DEPLOY_SERVER" "mkdir -p $DEPLOY_RELEASE_PATH/.next $DEPLOY_LOCATION/shared/logs $DEPLOY_LOCATION/shared/certs"

echo ""
echo "📤 Uploading standalone bundle to server..."

# Standalone bundle becomes the release root: server.js, trimmed node_modules, .next/server/, .next/*.json.
rsync -az --delete --progress -e ssh \
    .next/standalone/ "$DEPLOY_SERVER:$DEPLOY_RELEASE_PATH/"

# Static assets are NOT inside .next/standalone — Next expects them at .next/static.
echo "   Uploading .next/static..."
rsync -az --delete --progress -e ssh \
    .next/static/ "$DEPLOY_SERVER:$DEPLOY_RELEASE_PATH/.next/static/"

# Public assets aren't in standalone either — server.js serves them from ./public.
if [ -d "public" ]; then
    echo "   Uploading public/..."
    rsync -az --delete --progress -e ssh \
        public/ "$DEPLOY_SERVER:$DEPLOY_RELEASE_PATH/public/"
fi

# Ship nginx/ and systemd/ so `just setup-nginx-*` / `just setup-systemd` can read from current/.
if [ -d "nginx" ]; then
    echo "   Uploading nginx/..."
    rsync -az --delete --progress -e ssh nginx/ "$DEPLOY_SERVER:$DEPLOY_RELEASE_PATH/nginx/"
fi
if [ -d "systemd" ]; then
    echo "   Uploading systemd/..."
    rsync -az --delete --progress -e ssh systemd/ "$DEPLOY_SERVER:$DEPLOY_RELEASE_PATH/systemd/"
fi

# Certs live in shared/ (not the release) so cert renewals propagate without a redeploy.
# Each release symlinks `certs -> ../../shared/certs` in the finalise block below.
if [ -d "certs" ]; then
    echo "   Uploading certs/ to shared/..."
    rsync -az --progress -e ssh certs/ "$DEPLOY_SERVER:$DEPLOY_LOCATION/shared/certs/"
fi

echo "✅ Build artifacts uploaded"
echo ""

if [ -n "$ENV_PROD_FILE" ] && [ -f "$ENV_PROD_FILE" ]; then
    echo "📋 Copying environment file to server..."
    scp "$ENV_PROD_FILE" "$DEPLOY_SERVER:$DEPLOY_RELEASE_PATH/.env.production"
    echo "✅ Environment file copied"
fi

# Overwrite the standalone-shipped package.json so manual `npm start` falls back to the
# standalone server. The real port comes from the systemd unit's Environment=PORT line —
# that's the single source of truth so port changes never need a redeploy.
echo "📝 Writing release-root package.json with start script..."
ssh "$DEPLOY_SERVER" "cat > $DEPLOY_RELEASE_PATH/package.json" <<JSON
{
  "name": "$APP_NAME",
  "private": true,
  "scripts": {
    "start": "node server.js"
  }
}
JSON

echo ""
echo "🔗 Finalising release on server..."
ssh "$DEPLOY_SERVER" << REMOTE_SCRIPT
set -e

# Shared folder symlinks (logs, certs). Certs are symlinked so the nginx
# config's reference to `current/certs/*.pem` resolves to shared/certs/*.pem.
ln -sfn "$DEPLOY_LOCATION/shared/logs" "$DEPLOY_RELEASE_PATH/logs"
ln -sfn "$DEPLOY_LOCATION/shared/certs" "$DEPLOY_RELEASE_PATH/certs"
echo "✅ Shared folder symlinks created (logs, certs)"

# Atomic-ish current-pointer flip.
rm -f "$DEPLOY_LOCATION/current"
ln -s "$DEPLOY_RELEASE_PATH" "$DEPLOY_LOCATION/current"
echo "✅ Symlink current -> $DEPLOY_RELEASE_PATH"

# Keep last 5 releases.
cd "$DEPLOY_LOCATION/releases"
ls -t | tail -n +6 | xargs -r rm -rf
echo "✅ Old releases cleaned up (keeping last 5)"

# Restart the systemd service if present.
if systemctl list-unit-files | grep -q "^$APP_NAME.service"; then
    sudo systemctl restart "$APP_NAME"
    echo "✅ Service restarted"
elif [ -f "/etc/systemd/system/$APP_NAME.service" ]; then
    sudo systemctl daemon-reload
    sudo systemctl enable "$APP_NAME"
    sudo systemctl restart "$APP_NAME"
    echo "✅ Service enabled and restarted"
else
    echo "⚠️  No systemd service found. Install via 'just setup-systemd' then start manually."
fi

# Reload nginx if present.
if command -v nginx &> /dev/null; then
    sudo nginx -t && sudo nginx -s reload
    echo "✅ Nginx reloaded"
elif [ -f /opt/nginx/sbin/nginx ]; then
    sudo /opt/nginx/sbin/nginx -t && sudo /opt/nginx/sbin/nginx -s reload
    echo "✅ Nginx reloaded"
fi

REMOTE_SCRIPT

echo ""
echo "🎉 Deployment completed successfully!"
echo ""
echo "📁 Release structure:"
echo "   $DEPLOY_LOCATION/"
echo "   ├── current -> releases/$DEPLOY_TIMESTAMP"
echo "   ├── shared/"
echo "   │   ├── logs/"
echo "   │   └── certs/"
echo "   └── releases/"
echo "       └── $DEPLOY_TIMESTAMP/"
echo ""

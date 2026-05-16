#!/bin/bash
# Local deploy script: run tests, then push and deploy to Termux.
# Usage: bash scripts/deploy-local.sh
# Or via git alias: git deploy

set -e
cd "$(dirname "$0")/.."

echo "================================================"
echo "  Step 1: Running integration tests..."
echo "================================================"
go test -count=1 -v ./cmd/server/ -run TestFullIntegration 2>&1
echo ""
echo "  Tests passed!"

echo ""
echo "================================================"
echo "  Step 2: Pushing to remote..."
echo "================================================"
git push origin refinement

echo ""
echo "================================================"
echo "  Step 3: Deploying to Termux..."
echo "================================================"
ssh -p 8022 -i C:/Users/Gallan/.ssh/id_sim_lab_mi galaxy-a52s-5g.taila6b5cf.ts.net \
  'cd ~/lab_kom_sim && bash scripts/deploy.sh'

echo ""
echo "================================================"
echo "  Deploy complete!"
echo "================================================"
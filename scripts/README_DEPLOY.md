# Auto Deploy via SSH

## Setup Sekali di HP (Termux)

```bash
pkg install openssh -y
sshd
echo "sshd" >> ~/.bashrc
```

## Setup Sekali di Laptop

```powershell
# Generate SSH key
ssh-keygen -t ed25519 -f "$env:USERPROFILE\.ssh\id_ed25519" -N ""

# Copy key ke HP
type "$env:USERPROFILE\.ssh\id_ed25519.pub" | ssh galaxy-a52s-5g.taila6b5cf.ts.net "mkdir -p ~/.ssh && cat >> ~/.ssh/authorized_keys"

# Test SSH (harus tanpa password)
ssh galaxy-a52s-5g.taila6b5cf.ts.net "echo OK"

# Setup git alias
git config --global alias.deploy "!git push origin refinement && ssh galaxy-a52s-5g.taila6b5cf.ts.net 'cd ~/lab_kom_sim && ./scripts/deploy.sh'"
```

## Cara Pakai

```bash
git deploy
```

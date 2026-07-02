# Hosting on External Domain - Complete Setup Guide

## Overview
Deploy your P2P streaming app to a cloud server with a real domain (e.g., stream.example.com)

## Option 1: DigitalOcean Droplet (Easiest)

### 1. Create Droplet
- Sign up: https://www.digitalocean.com/
- Create → Droplets
- Choose: Ubuntu 22.04 LTS
- Size: $6/month (2GB RAM) minimum
- Region: Pick closest to you
- Authentication: SSH key (recommended)

### 2. SSH Into Server
```bash
# On your computer
ssh root@your_droplet_ip

# You'll see a terminal on the remote server
```

### 3. Install Go
```bash
cd /tmp
wget https://go.dev/dl/go1.22.2.linux-amd64.tar.gz
tar -C /usr/local -xzf go1.22.2.linux-amd64.tar.gz
echo "export PATH=$PATH:/usr/local/go/bin" >> ~/.bashrc
source ~/.bashrc
go version  # Verify
```

### 4. Upload Your Code
From your LOCAL computer:
```bash
scp -r c:\Users\jumai\Downloads\new-test\app root@your_droplet_ip:/root/vodapp
```

Or use Git:
```bash
# On server
cd /root
git clone https://your-repo-url/vodapp
cd vodapp
go build
```

### 5. Build & Run
```bash
# On server
cd /root/vodapp
go build
./vodapp -addr=:8080 &
```

### 6. Set Up Domain

**Buy Domain:** namecheap.com, godaddy.com, etc.

**Point to Server:**
- Go to DNS settings
- Add A record:
  - Name: @ (or subdomain like "stream")
  - Value: your_droplet_ip
  - TTL: 3600

Example: `stream.example.com` → points to `123.45.67.89`

### 7. Add Reverse Proxy (Nginx + SSL)
```bash
# Install Nginx
sudo apt update
sudo apt install nginx certbot python3-certbot-nginx

# Create config
sudo nano /etc/nginx/sites-available/stream

# Paste this (replace stream.example.com):
```
```nginx
server {
    listen 80;
    server_name stream.example.com;

    location / {
        proxy_pass http://localhost:8080;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_read_timeout 86400;
    }
}
```

### 8. Enable & Get SSL Certificate
```bash
# Enable config
sudo ln -s /etc/nginx/sites-available/stream /etc/nginx/sites-enabled/
sudo nginx -t  # Test
sudo systemctl restart nginx

# Get free SSL
sudo certbot --nginx -d stream.example.com

# Follow prompts, choose "Redirect to HTTPS"
```

### 9. Keep App Running (Systemd Service)
```bash
# Create service file
sudo nano /etc/systemd/system/vodapp.service

# Paste:
```
```ini
[Unit]
Description=VOD Streaming App
After=network.target

[Service]
Type=simple
User=root
WorkingDirectory=/root/vodapp
ExecStart=/root/vodapp/vodapp -addr=:8080
Restart=on-failure
RestartSec=10

[Install]
WantedBy=multi-user.target
```

```bash
# Enable and start
sudo systemctl daemon-reload
sudo systemctl enable vodapp
sudo systemctl start vodapp
sudo systemctl status vodapp  # Check status
```

### 10. Access Your App
```
https://stream.example.com/
https://stream.example.com/play/id
https://stream.example.com/obs/id
```

---

## Option 2: AWS Lightsail (Similar to DigitalOcean)

1. **Create Instance:** AWS Lightsail → Create Instance
2. **OS:** Ubuntu 22.04 LTS
3. **SSH to instance**
4. **Follow same steps as DigitalOcean above**

---

## Option 3: Heroku (Free but Limited)

```bash
# Install Heroku CLI
# Create Procfile in your app folder:
```
```
web: ./vodapp -addr=:$PORT
```

```bash
# Deploy
heroku create your-app-name
git push heroku main
heroku open
```

⚠️ **Note:** Heroku free tier limited, may sleep after inactivity

---

## Option 4: Your Own Server (VPS)

If you have a server:
```bash
# SSH to your server
ssh user@your-server.com

# Follow steps 3-5 from DigitalOcean section
# Then set up Nginx (steps 7-9)
```

---

## Network Detection for External Domain

Your network detection is **NOW FIXED** to handle external domains:

✅ **How it works:**
1. User connects via `https://stream.example.com`
2. Their browser sends request to Nginx (reverse proxy)
3. Nginx adds `X-Forwarded-For` header with their real IP
4. Your app reads this header
5. Shows correct: 🌐 External (even if external user)

### Verify It Works

After deployment, streamer from external network:
1. Opens `https://stream.example.com/play/id`
2. Browser console should show network detection
3. Should display: 🌐 External Network
4. OBS will see: network badge "🌐 External"

---

## Example Full Setup (copy-paste)

```bash
# SSH into Ubuntu 22.04 droplet
ssh root@YOUR_DROPLET_IP

# === Install Go ===
cd /tmp
wget https://go.dev/dl/go1.22.2.linux-amd64.tar.gz
tar -C /usr/local -xzf go1.22.2.linux-amd64.tar.gz
echo "export PATH=$PATH:/usr/local/go/bin" >> ~/.bashrc
source ~/.bashrc

# === Get Your Code ===
cd /root
# Option A: Git clone
# git clone https://your-repo vodapp
# Option B: Upload via scp from your computer

# === Build ===
cd /root/vodapp
go build

# === Install Nginx + SSL ===
sudo apt update
sudo apt install -y nginx certbot python3-certbot-nginx

# === Configure Nginx ===
cat > /tmp/nginx-config << 'EOF'
server {
    listen 80;
    server_name YOURDOMAIN.COM;
    location / {
        proxy_pass http://localhost:8080;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_read_timeout 86400;
    }
}
EOF

sudo cp /tmp/nginx-config /etc/nginx/sites-available/stream
sudo rm /etc/nginx/sites-enabled/default 2>/dev/null
sudo ln -s /etc/nginx/sites-available/stream /etc/nginx/sites-enabled/
sudo nginx -t
sudo systemctl restart nginx

# === Get SSL Certificate ===
sudo certbot --nginx -d YOURDOMAIN.COM

# === Create Systemd Service ===
sudo tee /etc/systemd/system/vodapp.service > /dev/null << 'EOF'
[Unit]
Description=VOD Streaming App
After=network.target

[Service]
Type=simple
User=root
WorkingDirectory=/root/vodapp
ExecStart=/root/vodapp/vodapp -addr=:8080
Restart=on-failure
RestartSec=10

[Install]
WantedBy=multi-user.target
EOF

sudo systemctl daemon-reload
sudo systemctl enable vodapp
sudo systemctl start vodapp

# === Check Status ===
sudo systemctl status vodapp
```

**Done! Your app is now live at `https://YOURDOMAIN.COM`**

---

## Firewall Rules (DigitalOcean Firewalls)

1. Go to Networking → Firewalls
2. Create firewall, add rules:
   - **Inbound HTTP** (port 80) from anywhere
   - **Inbound HTTPS** (port 443) from anywhere
   - **Outbound** all allowed
3. Apply to your droplet

---

## Monitor Your App

```bash
# SSH to server
ssh root@YOUR_DROPLET_IP

# Check logs
sudo journalctl -u vodapp -n 50 -f

# Check if running
sudo systemctl status vodapp

# Restart if needed
sudo systemctl restart vodapp

# View real-time server output
tail -f /var/log/syslog | grep vodapp
```

---

## HTTPS URLs

After domain setup:
```
Dashboard:  https://stream.example.com/
Streamer:   https://stream.example.com/play/<id>
OBS:        https://stream.example.com/obs/<id>
```

✅ All will be HTTPS (secure)
✅ Network detection works (shows 🌐 External)
✅ TURN servers handle NAT/firewall

---

## Cost Summary

| Provider | Cost | Ease | Performance |
|----------|------|------|-------------|
| DigitalOcean | $6-12/mo | ⭐⭐⭐⭐ | ⭐⭐⭐⭐ |
| AWS Lightsail | $3.50-5/mo | ⭐⭐⭐ | ⭐⭐⭐⭐ |
| Linode | $5-10/mo | ⭐⭐⭐⭐ | ⭐⭐⭐⭐ |
| Heroku | Free (limited) | ⭐⭐⭐⭐⭐ | ⭐⭐ |
| Your own server | $0 (hardware cost) | ⭐⭐ | Variable |

---

## Troubleshooting

### "Domain not resolving"
```bash
# Wait 24-48 hours for DNS
# Or check: nslookup stream.example.com
```

### "SSL certificate not working"
```bash
# Renew certificate
sudo certbot renew --force-renewal

# Or regenerate
sudo certbot --nginx -d stream.example.com --force-renewal
```

### "App crashed"
```bash
# Check logs
sudo journalctl -u vodapp -n 100

# Restart
sudo systemctl restart vodapp
```

### "Network shows Local instead of External"
✅ **FIXED** - Now reads X-Forwarded-For from Nginx
- Make sure Nginx proxy_set_header X-Forwarded-For is in config
- App will auto-detect external IPs

---

## Next: Custom Domain with Email

To add email (optional):
1. Set MX records in DNS
2. Use service like Mailgun or SendGrid
3. Update app to send notifications

---

**Ready to deploy?** Pick a provider and follow the steps above! 🚀

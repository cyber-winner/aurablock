module.exports = {
  apps: [{
    name: "aurablock",
    script: "/usr/local/bin/aurablock",
    args: "-dns-addr=0.0.0.0:53 -api-port=8082 -db-path=aurablock.db",
    cwd: "/home/cyber/CODES/aurablock/backend",
    watch: false,
    autorestart: true,
    restart_delay: 5000
  }]
};

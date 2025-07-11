# NixOS Docker Setup for flowctl

## Quick Fix

Run flowctl with sudo:
```bash
sudo ./bin/flowctl sandbox start --use-system-runtime --services examples/sandbox.yaml --pipeline examples/sandbox-pipeline.yaml
```

## Permanent Fix

Add yourself to the docker group in your NixOS configuration:

### Option 1: In configuration.nix

```nix
# /etc/nixos/configuration.nix
{
  # Enable Docker
  virtualisation.docker.enable = true;
  
  # Add your user to the docker group
  users.users.tillman = {
    extraGroups = [ "docker" ];
  };
}
```

Then rebuild and switch:
```bash
sudo nixos-rebuild switch
```

### Option 2: Using home-manager

```nix
# In your home.nix
{
  # This assumes docker is already enabled system-wide
  systemd.user.services.docker-users-setup = {
    Unit = {
      Description = "Add user to docker group";
      After = [ "docker.service" ];
    };
    Install = {
      WantedBy = [ "default.target" ];
    };
  };
}
```

### Option 3: Rootless Docker (Recommended for Security)

```nix
# /etc/nixos/configuration.nix
{
  virtualisation.docker = {
    enable = true;
    rootless = {
      enable = true;
      setSocketVariable = true;
    };
  };
}
```

## After Configuration Changes

1. Rebuild your system:
   ```bash
   sudo nixos-rebuild switch
   ```

2. Log out and log back in (or reboot) for group changes to take effect

3. Verify you're in the docker group:
   ```bash
   groups | grep docker
   ```

4. Test docker access:
   ```bash
   docker ps
   ```

5. Run flowctl:
   ```bash
   ./bin/flowctl sandbox start --use-system-runtime --services examples/sandbox.yaml --pipeline examples/sandbox-pipeline.yaml
   ```

## Alternative: Use nerdctl with containerd

If you prefer not to use Docker, you can install nerdctl:

```nix
# /etc/nixos/configuration.nix
{
  environment.systemPackages = with pkgs; [
    nerdctl
    containerd
  ];
  
  virtualisation.containerd.enable = true;
}
```

Then use flowctl with the default containerd backend:
```bash
./bin/flowctl sandbox start --use-system-runtime --services examples/sandbox.yaml --pipeline examples/sandbox-pipeline.yaml
```
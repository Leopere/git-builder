# Login Items & Extensions — Audit

Quick reference for what each item does and whether to keep it.

## Safe to disable or remove

| Item | Why |
|------|-----|
| **git-builder** (one or both) | This repo’s daemon. If you don’t need it running at login, uninstall: `git-builder -uninstall` (or from repo: `go run . -uninstall`). Two entries = likely installed/run from two paths; one is enough. |
| **bash** / **sh** | “Unidentified developer” — usually a script or wrapper added as a login item. If you don’t recall adding it, disable; you can re-enable if something breaks. |
| **GoogleUpdater** | Only needed if you want Chrome (or other Google apps) to auto-update at login. Safe to disable if you update manually. |
| **Greenshot** | Screenshot tool. Disable if you don’t use it at login. |
| **Zoom** / **zoom.us** (already disabled) | Leave disabled unless you need Zoom to start with the system. |

## Review / keep unless you don’t use them

| Item | Why |
|------|-----|
| **cloudflared** | Cloudflare Tunnel / WARP. Keep if you use them; disable if not. |
| **Mozilla VPN** | VPN. Keep if you use it; disable if not. |
| **WireGuard** | VPN. Same as above. |
| **OrbStack** | Docker/Linux on Mac. Keep if you use it. |
| **Twocanoes Software, Inc.** | Often WinClone or similar. Disable if you don’t use their apps. |

## Already disabled (no change needed)

- Benjamin Fleischer  
- USB-Scale-API  
- Zoom Video Communications, Inc.  
- zoom.us  

## Reducing git-builder to one entry

1. Uninstall the service (and one copy of the binary):  
   `git-builder -uninstall` or `go run . -uninstall`
2. In **System Settings → General → Login Items & Extensions**, turn off both **git-builder** entries (or remove them if the UI allows).
3. If you still want git-builder to run as a service, install once from one place:  
   `go run . -install` or run the installed `git-builder -install`. That should create a single login/background permission.

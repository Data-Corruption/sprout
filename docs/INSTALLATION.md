# Installation Guide Template

*This file is a template. You should update it to provide instructions for **your** users once you have configured the project.*

Replace `<YOUR_RELEASE_URL>` with the URL to your release tarball.  
Replace `<YOUR_APP_NAME>` with the name of your app.

---

## Installation

### Linux

To install the latest version, run the following command:

```sh
curl -fsSL <YOUR_RELEASE_URL>install.sh | sh
```

### Windows (WSL)

For Windows users running WSL, use PowerShell to bridge the environment:

```powershell
Set-ExecutionPolicy Bypass -Scope Process -Force; iex "& { $(irm <YOUR_RELEASE_URL>install.ps1) }"
```

> [!IMPORTANT]
> WSL can be a bit finicky. If you run into issues, try running `wsl --update` and after it completes, try running the install command again.

---

## Uninstall

To uninstall the app, simply run:

```sh
<YOUR_APP_NAME> uninstall
```
# Installation Guide Template

*This file is a template. You should update it to provide instructions for **your** users once you have configured the project.*

---

## Installation

### Linux

To install the latest version, run the following command:

```sh
# Replace with your raw install script URL
curl -fsSL https://raw.githubusercontent.com/YOUR_USERNAME/YOUR_REPO/main/scripts/install.sh | sh
```

### Windows (WSL)

For Windows users running WSL, use PowerShell to bridge the environment:

```powershell
# Replace with your raw install script URL
Set-ExecutionPolicy Bypass -Scope Process -Force; iex "& { $(irm https://raw.githubusercontent.com/YOUR_USERNAME/YOUR_REPO/main/scripts/install.ps1) }"
```

---

## Uninstall

To uninstall the app, run:

```sh
YOUR_APP_NAME uninstall
```
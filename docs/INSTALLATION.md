# Installation Guide Template

*This file is a template. You should update it to provide instructions for **your** users once you have configured the project.*

Replace `<YOUR_RELEASE_URL>` with the same URL you set in the build script.  
Replace `<YOUR_APP_NAME>` with the name of your app.

---

## Installation

To install the latest version, run the following install command:

**Linux**
```sh
curl -fsSL <YOUR_RELEASE_URL>install.sh | sh
```

**Windows**
```powershell
Set-ExecutionPolicy Bypass -Scope Process -Force; iex "& { $(irm <YOUR_RELEASE_URL>install.ps1) }"
```

> [!IMPORTANT]
> **Windows/WSL Support is Experimental.** 
> The Windows installer uses WSL to run the application. While functional, it may be finicky on some systems. If you run into issues, try running `wsl --update` and then re-run the installer.

---

## Uninstall

To uninstall the app, simply run:

```sh
<YOUR_APP_NAME> uninstall
```
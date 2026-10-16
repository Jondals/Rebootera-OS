# Changelog

All notable changes to Rebootera OS. Dates use the format YYYY-MM-DD.
Spanish version: [CHANGELOG.es.md](CHANGELOG.es.md).

## [0.2.0] - 2026-10-16

### Added

- **New interface** based on the "Obsidian Aurora" design: sidebar navigation, top bar with live status (boot media, network, Secure Boot, clock), and a dashboard with real hardware data.
  - Overview: hero, install card, shortcuts, detected hardware profile (computer, internal disk with its partition layout, boot media, firmware security with TPM) and three live widgets (temperatures from the hardware sensors, network throughput, kernel log).
  - Driver Assistant: every PCI and USB device with its Linux driver, its Windows hardware ID (`PCI\VEN_…&DEV_…`) and the devices needed to install (storage and network) highlighted.
  - USB & Storage: disks and partitions with their role (EFI, Windows, recovery, Rebootera Vault…), file system, usage and the Safe Disk Guard protection.
  - Installation Wizard in six steps: choose an image, Windows 11 / Linux requirement check, critical drivers, target disk, execution plan and launch.
  - Settings: language, accent color (Aurora teal or Nebula violet), startup intro and reduced motion.
- **English and Spanish** interface, switchable from the top bar.
- **Animated intro**: aurora background, the logo drawn stroke by stroke, the name letter by letter and a progress bar that waits for real system data. Skippable and respects reduced motion.
- **Boot splash** (Plymouth) and **boot menu** (GRUB and Ventoy) with the Rebootera identity: logo, halo, Inter and JetBrains Mono fonts.
- **Rebootera USB** (`scripts/make-usb.sh`): Ventoy 1.1.18 (downloaded from its official release and checked against its published SHA-256) plus the exFAT "REBOOTERA" Vault with `isos/`, `drivers/`, `tools/`, `files/` and Rebootera OS as the default boot entry.
  - Ventoy configured safely: `VTOY_SECURE_BOOT_POLICY=1`, Windows 11 requirement bypasses disabled (`VTOY_WIN11_BYPASS_CHECK=0`, `VTOY_WIN11_BYPASS_NRO=0`).
- **Persistent storage**: the Vault is mounted at boot (also when Ventoy keeps the partition busy, through `VentoyPart`) without executables, setuid or devices. Settings survive reboots.
- **OS Library**: images on the USB grouped by category, SHA-256 integrity check, verification state (verified, imported, not checked, damaged) and safe deletion.
- **Download Center**: official catalog (Ubuntu 26.04 and 24.04 LTS, Debian 13, Fedora 44, Arch Linux, Alpine Linux and Windows 11 as a manual download). Every download resolves the latest file and its SHA-256 from the official checksum index over HTTPS, can be paused and resumed (also after a reboot) and only reaches the library when its checksum matches.
- **Add images from another drive**: the drive is opened read-only (without replaying journals) and only folders and images are listed.
- **Start an installer**: the wizard preselects the chosen image in the Ventoy menu and restarts; when Rebootera starts again, it becomes the default entry again.
- **Rebootera Desktop** (`out/Rebootera Desktop.exe`) for Windows (and Linux through the browser): manage a Rebootera USB without booting from it. Detects the USB, shows its version and space, adds images with the native file dialog, downloads official images, updates Rebootera OS on the USB keeping the previous version as a fallback, and creates a new Rebootera USB on Windows with the official Ventoy tool. Own icon, dark title bar and high-DPI support.
- New VM tests: `scripts/vmtest-usb.py` (end-to-end test of a Rebootera USB: Vault, real download, import, settings, boot of the chosen image and persistence after reboot) and `scripts/ui-preview.py` (interface screenshots with the same WebKitGTK as the image).

### Changed

- The interface now uses **pnpm** (pinned version `pnpm@12.9.1` through corepack in the build).
- The build leaves **only the final ISO** in `out/`; debug images stay inside the WSL build folder.
- Default language and boot menu in English (Spanish available).
- Comments in every file and function follow the Better Comments style (`*`, `!`, `?`, `TODO`).

### Fixed

- Mounts made by the agent were trapped in its private mount namespace (`PrivateTmp`/`ProtectHome`) and Core could not see the imported drive.
- `TestSinVault` depended on the build machine having a Vault.
- The "Create USB" step on Windows now checks the Rebootera OS image before erasing the drive.
- Large or non-empty USB disks (such as an external SSD) now show their volumes, the data they contain and a clear warning before they can be erased.

## [0.1.0] - 2026-10-16

### Added

- First bootable image (Debian 13, live-build): BIOS and UEFI, with Secure Boot through Debian's signed shim.
- Full-screen interface in a kiosk (`cage` + GTK4/WebKitGTK) that opens by itself.
- Privilege separation: unprivileged `rebootera-core` (local API on 127.0.0.1 with a per-boot token, Host check, anti-CSRF header and CSP) and `rebootera-agent` (root, closed list of operations, checks the caller through socket credentials).
- Real "This computer" page and power off / restart from the interface.
- QEMU/KVM test bench for UEFI, UEFI with Secure Boot and BIOS.

### Fixed

- Black screen: `pam_systemd` (systemd 257) gives sessions an ambient capability that makes WebKitGTK's sandbox (bubblewrap) abort.
- Secure Boot state was always "unknown" (wrong EFI variable GUID).
- Powering off took more than 90 seconds because of Plymouth on a live system.

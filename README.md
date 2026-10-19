# Rebootera OS

**Install, recover and manage operating systems from a single USB drive — no technical knowledge and no other operating system needed.**

Rebootera OS is a bootable environment with its own identity and interface. Plug the Rebootera USB into any computer, start from it, and a full-screen guided interface lets you download official images, install Windows or Linux, inspect the hardware and its drivers, and look after your disks. A companion app, **Rebootera Desktop**, manages the same USB from Windows without rebooting.

Spanish version: [README.es.md](README.es.md) · Changes: [CHANGELOG.md](CHANGELOG.md)

![Rebootera OS overview](docs/images/overview.png)

## Features

| Area | What it does |
| --- | --- |
| **Boots anywhere** | BIOS and UEFI, with Secure Boot through Debian's signed shim. Own boot menu, boot splash and animated intro. |
| **Overview** | Live hardware profile: computer, internal disks and their partitions, boot media, Secure Boot and TPM, temperatures, network throughput and kernel log. |
| **OS Library** | Images stored on the USB, grouped by category, with SHA-256 integrity checks. A damaged image is never offered for installation. |
| **Download Center** | Official images (Ubuntu, Debian, Fedora, Arch Linux, Alpine). The file and its checksum are resolved from the official checksum index over HTTPS; downloads resume after interruptions and only reach the library when the checksum matches. For Ubuntu, Debian and Fedora the checksum list is also verified with the project's official GPG key before anything is downloaded; for Arch Linux and Alpine, which sign each ISO, the image's own signature is verified once it is downloaded. |
| **Add images** | From another USB or disk (opened read-only) or, from Windows, with Rebootera Desktop. Any `.iso` copied to the `isos` folder also appears. |
| **Installation Wizard** | Requirement check (UEFI, Secure Boot, TPM 2.0, memory, disk), critical drivers and target disk, then starts the original installer through the boot menu. Optional: answer language, time zone and user account automatically (Windows and Debian; the disk is always chosen by you), take matching drivers to the Windows installer, skip the Windows 11 requirement check, and start the installer only once (UEFI). |
| **Driver Assistant** | Every PCI and USB device, the Linux driver in use and the Windows hardware IDs, highlighting what the installer needs (storage and network). It also matches those hardware IDs against the Windows driver packages (`.inf`) in the `drivers/` folder of the USB. |
| **Wi-Fi** | Connect the live session to a wireless network from Settings (WPA2/WPA3 and open networks, also hidden ones). |
| **USB & Storage** | Disks and partitions with their role and usage. **Safe Disk Guard**: internal disks and the Rebootera USB are protected and only read. |
| **Disks and partitions** | Every disk as a visual bar. Resize a partition, move the limit between two neighbors with a slider, merge, delete or create partitions (ext4 and NTFS can be resized without losing files). Each change shows the exact steps first and asks you to type the disk name; the USB Rebootera runs from is never touched. |
| **Rebootera Desktop** | Windows app with a single **Create USB** option: it builds a new Rebootera USB or updates the one you have (without erasing it), downloading the latest Rebootera OS for you, and puts your images (for example a Windows 11 LTSC ISO) and driver folders on it in the same step, so nothing has to be moved by hand. Overview and Driver Assistant show and manage what the USB carries. It also downloads official images even without a USB: they are saved in the `Rebootera` folder of your user. |
| **Languages** | English and Spanish. |

| | |
| --- | --- |
| ![Intro](docs/images/intro.png) | ![Boot menu](docs/images/boot-menu.png) |
| ![Download Center](docs/images/downloads.png) | ![Rebootera Desktop](docs/images/desktop.png) |

## Getting a Rebootera USB

**Download:** the public website and releases (repository `Jondals/Rebootera-OS-downloads`; the source code is private) always offer the latest build: `Rebootera-Desktop.exe` for Windows and `rebootera-os-amd64.iso`. Both are rebuilt on every push to `master` and published in the [`latest` release](https://github.com/Jondals/Rebootera-OS-downloads/releases/tag/latest).

**Channels:** `master` is the stable channel (release `latest`). The `beta` branch is created automatically from the first stable release and publishes the `beta` prerelease on every push to it: that is where changes are tried before merging them into `master`.

**From Windows** — run `Rebootera Desktop.exe` and open **Create USB** (it is the first option). Choose where Rebootera OS comes from — *download the latest version* (recommended, checked against its published SHA-256 before anything is erased) or *use a Rebootera ISO from this PC* — then the USB drive, and type the confirmation. Windows asks for administrator permission to prepare the drive. Everything on the chosen drive is erased; disks that contain data are clearly marked. If your USB already has things (for example it was made with Rufus), choose **"keep the files"**: Rebootera is added next to them and nothing is erased. Before confirming you can **add images and driver folders** (your Windows 11 LTSC, a Linux, your network drivers): they are copied in the same step. If you already have a Rebootera USB, choose it in the list instead: nothing is erased, Rebootera OS can be updated and what you add is copied.

**From Linux** — as root:

```bash
scripts/make-usb.sh --device /dev/sdX --confirm /dev/sdX --iso out/rebootera-os-0.10.0-amd64.iso
```

The script refuses non-removable disks and the system disk.

**Without persistence** — the ISO is hybrid, so it can also be written directly to a USB stick (for example with Rufus or `dd`) or booted from a DVD. Rebootera OS then runs read-only: no library and no saved settings.

### Starting the computer from the USB

1. Plug in the USB and restart the computer.
2. Open the boot menu right after switching on (usually F12, F11, F8, F9 or Esc) and choose the USB.
3. Rebootera OS starts after a few seconds.

With Secure Boot enabled, the first start on each computer shows a blue MokManager screen to enroll the boot menu key: choose **Enroll key from disk**, open the **VTOYEFI** drive, select `ENROLL_THIS_KEY_IN_MOKMANAGER.cer`, confirm and reboot. This happens once per computer.

> **Security trade-off:** Ventoy's policy 1 (which refuses images without a verified signature) does not let Linux start with Secure Boot, so Rebootera uses policy 0. Once you enroll the key, that computer will also start images whose signature has not been checked. Enroll it only on computers you trust, and remove it from MokManager (**Delete MOK**) when you no longer need it.

### What is on the USB

```
REBOOTERA (exFAT)
├─ isos/windows   Windows installation images
├─ isos/linux     Linux installation images
├─ isos/tools     Rescue and utility images
├─ drivers/       Driver packages
├─ tools/         Portable tools
├─ files/         Your files
├─ rebootera/     Rebootera OS (do not modify)
└─ ventoy/        Boot menu settings (do not modify)
```

## Security

- The interface has no privileges. **Rebootera Core** runs as an unprivileged user and serves its API only on `127.0.0.1`, protected by a per-boot token, a Host check, an anti-CSRF header and a strict Content Security Policy.
- Only **Rebootera Agent** runs as root, and it accepts a closed list of operations from Core's user only (checked through the socket credentials).
- Other drives are always opened **read-only**, without replaying file system journals.
- Downloads come only from official sources over HTTPS and are checked against the official SHA-256 before they reach the library. Where the project signs its checksum list (Ubuntu, Debian, Fedora), the OpenPGP signature is verified against the project's pinned key first, and the download is refused if it does not match; where it signs each ISO (Arch Linux, Alpine), the image is verified after downloading and deleted if the signature does not match.
- Wi-Fi passwords never appear on a command line: they go to a root-only (0600) NetworkManager profile that disappears when you disconnect or shut down.
- The files that answer the installer (including an encoded password) live on the USB only until Rebootera starts again, and are then deleted.
- The boot menu does not hide its security trade-off: with Secure Boot, enrolling Ventoy's key lets that computer start images without a verified signature (the interface and this README say so), and the Windows 11 requirement bypasses are off.

Known limitations are listed in [docs/LIMITACIONES.md](docs/LIMITACIONES.md) (Spanish).

## Website and automatic builds

- `site/` is the landing page (plain HTML, English and Spanish). `.github/workflows/pages.yml` publishes it to GitHub Pages; in the repository settings, **Pages → Source** must be set to *GitHub Actions* once.
- `.github/workflows/build.yml` builds the ISO and `Rebootera-Desktop.exe` inside a Debian 13 container on every push to `master` and replaces the `latest` release with them in the public downloads repository (secret `RELEASES_TOKEN`). `.github/workflows/site-sync.yml` copies the website there too. The page reads the version, date and SHA-256 of that release, and Rebootera Desktop downloads the ISO from it. The first build only exists after the first push.

## Building

Everything is built on Windows inside a dedicated Debian 13 WSL distribution called `RebooteraBuild`. The commands below assume the project is in `D:\dev\Rebootera OS`; change `/mnt/d/dev/Rebootera OS` if it is somewhere else. Both builds need an Internet connection (Debian packages, Go modules and pnpm packages).

### 1. Prepare the build environment (once)

Open PowerShell and run:

```powershell
wsl --install Debian --name RebooteraBuild --location D:\WSL\RebooteraBuild --no-launch
wsl -d RebooteraBuild -u root --exec bash -c "apt-get update && apt-get install -y live-build debootstrap rsync curl golang nodejs node-corepack imagemagick librsvg2-bin grub-common fonts-inter fonts-jetbrains-mono xorriso mtools squashfs-tools dosfstools exfatprogs exfat-fuse gdisk parted qemu-system-x86 qemu-utils ovmf xvfb gir1.2-webkit-6.0 python3-gi"
```

The last six packages are only needed to run the virtual machine tests.

### 2. Build the ISO

```powershell
wsl -d RebooteraBuild -u root --exec bash "/mnt/d/dev/Rebootera OS/scripts/build.sh"
```

It takes 10–20 minutes. The script runs the Go tests, builds `rebootera-core` and `rebootera-agent`, builds the interface with pnpm, generates the boot menu and splash art and runs live-build. The result is:

- `out/rebootera-os-<version>-amd64.iso` and its `.sha256`. Older ISOs in `out/` are deleted, so `out/` only ever holds one ISO.

For a test image with a root console on the serial port, add `--debug`. That image stays inside WSL in `/root/rebootera-build/out/` and never mixes with the final one.

### 3. Build the Windows app

```powershell
wsl -d RebooteraBuild --exec bash "/mnt/d/dev/Rebootera OS/scripts/build-desktop.sh"
```

It takes about a minute. The script builds the interface, embeds it together with the files of a new Rebootera USB (boot menu settings and theme), adds the icon and the Windows manifest, and cross-compiles. The result is:

- `out/Rebootera Desktop.exe`: a single file, no installation. It needs the Microsoft Edge WebView2 Runtime, which Windows 10 and 11 already include. Close the app before rebuilding it.

### 4. Make a Rebootera USB

- **Real USB drive**: open `Rebootera Desktop.exe` → **Create USB** and choose the ISO from step 2, or use `scripts/make-usb.sh --device` on Linux (see [Getting a Rebootera USB](#getting-a-rebootera-usb)).
- **Disk image for virtual machines**: `wsl -d RebooteraBuild -u root --exec bash "/mnt/d/dev/Rebootera OS/scripts/make-usb.sh" --image /root/usb.img --iso "/mnt/d/dev/Rebootera OS/out/rebootera-os-<version>-amd64.iso"`.

### Interface development (pnpm)

```powershell
cd ui
pnpm install
pnpm run format:check
pnpm run check-types
pnpm run build
```

## Testing

| Script | What it checks |
| --- | --- |
| `go test ./...` (in `core/`) | API protections, storage and hardware inventory, catalog resolvers, downloads (resume, checksum mismatch), library, boot selection, desktop kit |
| `go test -tags online ./internal/catalog/` | Every catalog entry against the real official sources |
| `scripts/test-all.sh [--quick] [--no-build]` | **Everything at once**: builds both ISOs and runs every VM test in parallel (three 1.5 GB VMs at a time). About 8 minutes of tests; `--quick` boots the two ISOs in 1 minute |
| `scripts/cpu-probe.py URL` | Idle CPU of each interface page with the same WebKitGTK as the image (catches endless animations) |
| `scripts/vmtest.py MODE ISO OUT` | Boots the ISO in QEMU/KVM (`uefi`, `uefi-sb`, `bios`): services, isolation, permissions, Secure Boot, interface screenshots, power off from the interface |
| `scripts/vmtest-usb.py MODE USB.img DISK.qcow2 OUT` | End-to-end Rebootera USB: Vault, real download, import from another disk, settings, boot of the chosen image and persistence after reboot |
| `scripts/vmtest-v03.py USB.img OUT` | 0.3.0 features end to end: GPG-verified download, driver matching with real virtio-win INF files, Wi-Fi with simulated radios and a virtual access point, Debian preseed with boot-once, Windows preparation and cleanup |
| `scripts/test-desktop-windows.ps1` | Windows only: runs the real `Rebootera Desktop.exe` against a **fake** USB (a temporary folder) and checks that the image and drivers are copied byte for byte, the system is untouched and bad requests are refused |
| `scripts/make-partman-disk.sh OUT.qcow2` | Test disk for the partition manager: ext4 + NTFS with data and checksums (only image files and a loop device) |
| `REBOOTERA_LOOP_TESTS=1 go test ./cmd/rebootera-agent -run TestLoop` | As root in WSL: runs the partition manager on real loop disks (resize, move a limit in both directions, merge, create, delete) and checks files by hash and file system health |
| `scripts/vmtest-interactive.py MODE IMG OUT` | Drives a VM from outside (keys, clicks, screenshots), for example to walk through MokManager |
| `scripts/ui-preview.py URL OUT` | Interface screenshots with the same WebKitGTK as the image |

Results are recorded in [docs/PRUEBAS.md](docs/PRUEBAS.md) (Spanish).

## Project layout

| Folder | Contents |
| --- | --- |
| `branding/` | Logo |
| `core/` | Go: `rebootera-core`, `rebootera-agent`, `rebootera-desktop` and their packages |
| `ui/` | Rebootera UI (TypeScript, no runtime dependencies) |
| `shell/` | Kiosk window (GTK4 + WebKitGTK) running inside `cage` |
| `os/` | live-build configuration, services, boot menu and splash, Rebootera USB contents |
| `scripts/` | Build, USB creation, boot art and test scripts |
| `docs/` | Technical plan, test results and limitations (Spanish) |

## Third-party components

Rebootera OS is built on Debian GNU/Linux. The Rebootera USB uses [Ventoy](https://www.ventoy.net) (GPLv3) as its multiboot layer, downloaded from its official release and verified with its published SHA-256. Rebootera Desktop uses [go-webview2](https://github.com/jchv/go-webview2) (MIT). Fonts: Inter and JetBrains Mono (SIL Open Font License).

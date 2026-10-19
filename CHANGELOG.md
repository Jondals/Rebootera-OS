# Changelog

All notable changes to Rebootera OS. Dates use the format YYYY-MM-DD.
Spanish version: [CHANGELOG.es.md](CHANGELOG.es.md).

## [0.8.0] - 2026-10-19

### Changed

- **All menus redone to be simple.** One calm look everywhere: flat colors, normal lettering instead of small technical capitals, bigger text and buttons, and far less on screen.
  - **Home** is one question ("What do you want to do?") and big cards: Install a system, Download systems, My images, Disks, Drivers (in Rebootera Desktop: your USB at a glance and Make USB, Download, My images, Drivers). Processor, disks and firmware moved to a folded "About this computer"; the temperature, network and kernel-log widgets are gone from the front page.
  - **Top bar** only shows a notice when something is wrong (no network, no USB). The clock, boot mode and Secure Boot chips are gone.
  - **Menu names** are plain: Home, Install, My images, Drivers, Download, Disks.
  - **Download** shows one small card per system: name, one line, a drop-down to pick the version (with its size) and a Download button. Verification, boot compatibility and the SHA-256 are behind "Details". The bottom bar only appears while something is downloading; the queue and bandwidth are buttons in the header.
  - **Install** has four steps (System, Disk, Options, Install) instead of six; the hardware and driver checks are folded inside Options, and the long safety banner is gone (the subtitle says nothing is written until you confirm).
  - **Drivers** lists devices with just a name, a kind and "Has a driver" / "No driver"; identifiers appear when you hover. The USB driver matches are folded.
  - **Make USB**: the source of Rebootera OS and the optional images and drivers are folded; only the USB and the final button stay open.
  - **Settings**: language, keyboard and Wi-Fi first; accent color, intro, reduced motion and system info are under "More options".
  - **My images**: shorter texts and softer rows.

## [0.7.0] - 2026-10-18

### Added

- **Many more systems, 40 images in 21 families.** Windows 11/10 and Server 2022/2025 (evaluation images download directly from Microsoft), Ubuntu (Desktop, Server, Kubuntu, Xubuntu, Lubuntu, Ubuntu MATE), Debian (netinst and GNOME/KDE/Xfce live), Fedora (Workstation, Xfce, Cinnamon, Server), Linux Mint (Cinnamon, Xfce, MATE), Arch, Alpine, openSUSE Tumbleweed, Kali, Rocky, AlmaLinux, Proxmox VE, FreeBSD, OpenBSD, SystemRescue and Rescuezilla. Where a project signs its checksums (Ubuntu, Debian, Fedora, Mint, Kali, AlmaLinux) the OpenPGP signature is verified with a pinned key; every other image is checked against the checksum the project publishes over HTTPS. New resolvers for BSD, GitHub releases, SourceForge and direct links. Every downloadable entry was resolved against the real servers (Windows 11 and 10 Home/Pro stay manual, from Microsoft's page).
- **Installers made with Rufus (or any tool that extracts Windows into folders) become ISOs.** Rebootera finds Windows installer folders on your drives (root of each drive and its first-level folders; in Rebootera OS on the opened media), and "Make ISO" repackages one into a bootable ISO with both BIOS and UEFI boot (and UDF for files over 4 GB such as `install.wim`) straight into the library, as a normal job in the queue with progress. Rebootera Desktop also lets you choose a folder by hand. The source folder is only read. Linux uses `genisoimage` and Windows uses IMAPI (built into Windows); both were tested for real (ISO 9660 signature, El Torito catalog with BIOS and EFI platforms, volume label).
- **Multi-connection downloads.** Large images are downloaded in 1 MiB blocks over several connections at once (up to 8, 4 by default) with a block map, so a cut resumes where it stopped, and the hash is still computed in order. **Adjust bandwidth** (Download Center) sets the number of connections and a speed limit that applies at once, even to a download in progress, and is remembered on the USB.
- **Direct to the USB.** Downloads already go straight to the USB; now "Update Rebootera OS (latest)" in Rebootera Desktop also downloads the system image directly into the USB (as `.new`, so the version already there stays intact until the SHA-256 matches the published one), resumes if cut, and does nothing if the USB already has that version. The image is no longer downloaded to the PC's disk first when updating.

### Changed

- **New Download Center**: one card per family with selectable versions (tags such as Recommended, LTS, Light, Live; size on each), a search box, tabs (All, Windows, Linux, Servers & BSD, Rescue & tools), a status card for the destination USB (name and free space), a bottom bar with the active download, **View queue** and **Adjust bandwidth**, copy SHA-256 for each image and a clear "already on your USB" state.
- **Library** shows the installer folders found and their "Make ISO" button.
- Rebootera Desktop opens links to the vendors' pages in your browser from the app.

### Fixed

- The catalog check no longer fails on images with no published checksum (Microsoft evaluation images): they are imported as "not verified" and the interface says so.
- `kali-linux-live` removed: Kali no longer hosts the live images for direct download (only torrents).

## [0.6.0] - 2026-10-18

### Added

- **Partition manager** (Disks and partitions, in Rebootera OS): every disk is a visual bar with one piece per partition and per free gap. Pick a piece and you can **resize** it, **share space with its neighbor** (move the limit between two partitions with a slider), **merge** two neighbors, **delete** or **create** a partition in a free gap. Every action shows how the disk will look, the exact steps, what gets erased and what gets copied, and asks you to type `CHANGE <disk>`. Works on ext4 and NTFS (the file systems that can be resized without losing files); exFAT and FAT32 can be merged away, deleted and created. Moving a limit copies the data of the partition on the right, with progress and a warning not to unplug or switch off. The USB Rebootera runs from, optical drives, disks with open partitions, read-only disks and MBR disks with extended partitions are never touched; the agent re-checks everything (it does not trust the interface) and logs each operation.
- **Add Rebootera to a USB that already has things, without erasing it** (for example one made with Rufus): Create USB offers it next to "new USB". It uses Ventoy's non-destructive install (Windows shrinks the volume a little if the drive is full), keeps the files where they are and shows the images (ISO) it already holds in the boot menu. Rebootera OS accepts an NTFS Vault (mounted with the kernel's `ntfs3` driver) besides exFAT. The Rufus installer boot of that drive is replaced by the Rebootera menu.

### Changed

- **Download systems** is simpler: a short list of official images, one button per row and a size on the right; transfers appear only when there are some; details (boot compatibility, signature, Secure Boot) are folded behind an info button; filter by Linux, Windows or tools. The latest version and size are looked up in the background and Core keeps them for 10 minutes, so the page opens at once.
- **Faster copies to the USB.** Reading and writing now happen at the same time with 4 MiB buffers (before, each chunk was read, written and hashed one after another), the Rebootera OS image is no longer read back after copying (the system served it from its cache, so it checked nothing and doubled the time; the size is checked after flushing), and the library listing is cached for 3 seconds.
- **Lighter Rebootera Desktop.** At rest the interface took about 40 % of a core (WebView2 animating the background and blurs): Desktop now always uses the light mode, polls only the USB every 5 seconds (no sensors, disks, kernel log or hardware, which do not exist on Windows) and PowerShell no longer flashes a window (every helper runs hidden).
- **Install is the second tab** of Rebootera OS (Overview, Install, Library, Drivers, Download, Disks).
- Keyboard layouts: only Spanish and English.

## [0.5.0] - 2026-10-17

### Changed

- **Rebootera Desktop has one way to write a USB: Create USB.** "My USB" is gone. Create USB now does both things: a new USB drive is erased and turned into a Rebootera USB, and a Rebootera USB you already have is **updated without erasing anything** (it shows up in the same list, marked as safe). The system can be kept, downloaded again (latest) or taken from an ISO.
- **Overview** and **Driver Assistant** are back in Rebootera Desktop. Overview shows your Rebootera USB at a glance (version, space, images, driver packages) with shortcuts; Driver Assistant lists the driver packages stored on the USB grouped by folder with the state of their catalog signature, and adds or removes driver folders.

### Added

- **Private code, public downloads.** The source stays in the private repository; the releases (`latest` and `beta`) and the website are published in the public repository `Jondals/Rebootera-OS-downloads` with the secret `RELEASES_TOKEN`. Rebootera Desktop downloads from there.
- **Stable and beta channels.** Every push to `master` publishes the `latest` release and, if the `beta` branch does not exist, creates it from that commit (or moves it forward when it has no commits of its own) and publishes the `beta` prerelease with the same files. Pushes to `beta` build and publish only the `beta` prerelease, so changes can be tried before they reach `master`.
- **Put your things on the USB in the same step.** Create USB has a new step to add **images** (several ISO/IMG/WIM/VHD files at once, for example a Windows 11 LTSC; each goes to `isos/windows`, `isos/linux` or `isos/tools`, guessed from the name and editable) and **driver folders**. They are copied right after Rebootera OS, with progress per file, so the USB is ready to boot and use with nothing to move by hand. Files are written as `.part` and renamed when complete, so a cut never leaves a half image under its final name.
- Everything is validated before touching the USB: a missing file, a category that does not exist, a repeated image or a driver folder without any x64 `.inf` package is refused before anything is erased or changed. From a driver folder only driver files are copied: installers (`.exe`, `.msi`), archives and disk images are skipped.
- Native Windows dialogs to pick several images and to pick a folder.
- Tests: Go tests for validation, copying, system update and driver removal (also run on Windows), a Windows test of the multi-select dialog result, and `scripts/test-desktop-windows.ps1`, which drives the real `.exe` against a **fake** USB (a temporary folder) and checks the copied files by hash.

## [0.4.0] - 2026-10-17

### Added

- **Signed images for Arch Linux and Alpine**: both projects sign every ISO (not their checksum list), so after the SHA-256 matches, Rebootera downloads the detached OpenPGP signature (`.sig` for Arch, `.asc` for Alpine) and verifies the whole image, streaming it, against the project's pinned key (fingerprints checked against archlinux.org/download and alpinelinux.org/downloads). An image whose signature does not match is deleted, like one with a bad checksum. The Download Center shows a "GPG-signed image" badge. Checked against the real Arch 2026.10.01 and Alpine 3.24.2 images.
- **Keyboard layout** in Settings (Spanish and English). Core asks the agent, which accepts only that closed list, writes the layout where `start-kiosk` reads it and restarts the interface (cage ignores SIGTERM, so it is killed and systemd reopens it in 2 seconds). The choice is saved in the Vault and applied again at the next start. Before, the layout was fixed to Spanish.
- **Continuous checks** (`.github/workflows/test.yml`): `gofmt`, `go vet` and `go test` for the core, type check and Prettier for the interface, on every push and pull request.
- `scripts/vmtest-v03.py` has a keyboard check (the change restarts the kiosk and cage starts with the new `XKB_DEFAULT_LAYOUT`).

### Changed

- The catalog now says `signature: gpg` for Arch Linux and Alpine: there is no catalog entry with an unchecked signature left except Windows (manual download).

## [0.3.0] - 2026-10-16

### Added

- **Windows drivers** (milestones 5 and 6): Rebootera reads the `.inf` driver packages in the `drivers/` folder of the USB (UTF-16/UTF-8, string substitution, only x64 sections), matches their hardware IDs against the real devices of the computer (most specific ID first, newest package wins) and checks that each package has a signed catalog. The Driver Assistant and the wizard show the matches and the critical devices that still have no driver.
  - **Take the matching drivers to the Windows installer**: Rebootera packs the matching packages into a `.7z` that Ventoy extracts into the installer (`X:\rebootera-drivers`) and generates an `autounattend.xml` whose driver paths (`windowsPE` and `offlineServicing`) point there, so Windows Setup sees the disk and the network card before it asks for a disk and the drivers also end up in the installed system. Only `.inf` packages are used; nothing is run. The original ISO is never modified.
- **GPG-verified downloads**: for Ubuntu 26.04 and 24.04, Debian 13 and Fedora 44 the checksum list is verified with the project's OpenPGP key (pinned in the program, with fingerprints checked against the projects' own pages) before anything is downloaded. A download whose signature is missing or does not match is refused. The Download Center shows a "GPG-signed index" badge. Arch Linux and Alpine do not sign their checksum list and are marked as such.
- **Wi-Fi** in Settings: scan, connect (WPA2, WPA3 and open networks, also hidden ones) and disconnect. The password never appears on a command line: the agent writes a root-only (0600) NetworkManager profile and removes it when you disconnect or when the connection fails.
- **Answers for the installer** (Windows and Debian): language and keyboard, time zone and an optional user account, through `autounattend.xml` and Debian `preseed` handed over by Ventoy's `auto_install`. They never include disk configuration: you always choose the disk in the installer. The password is stored encoded (Windows) or as a SHA-512 hash (Debian) and, like every generated file, deleted when Rebootera starts again.
- **Skip the Windows 11 requirements check** (TPM, Secure Boot, RAM) as an explicit option in the wizard, active only for that installation.
- **Start the installer only once** (UEFI, optional): before restarting into the installer, the agent puts the USB last in the firmware boot order and sets it as `BootNext`; when Rebootera starts again the original order is restored (keeping any new entries the installed system added). This avoids the installer starting again after its first restart.
- **Rebootera Desktop**: **Create USB** is now the first (and only) way to make a USB. It can **download the latest Rebootera OS for you** (checked against its published SHA-256 before the drive is touched) or use an ISO from the PC; "My USB" no longer creates anything, it only manages an existing USB (add images, **download and install the latest** Rebootera OS or choose an ISO). Without a Rebootera USB, **downloading systems now works**: images are saved in the `Rebootera` folder of your user (before, the Download Center was disabled until a USB was connected). Sessions started from read-only media (a DVD or a virtual machine) explain why there is no storage and how to get it.
- **Website** (`site/`, English and Spanish) with the download buttons for `Rebootera-Desktop.exe` and the ISO, and **GitHub Actions** that rebuild both on every push to `master` and publish them in the `latest` release (`.github/workflows/`).
- `scripts/cpu-probe.py` measures the idle CPU of every page (and lists the active CSS animations) with the same WebKitGTK as the image.
- `scripts/vmtest-v03.py` (end-to-end test of all of the above, in three independent scenarios: Wi-Fi, Debian and Windows), `scripts/usb-add-drivers.sh` and a diagnostic service in debug images that dumps the graphical state over the serial port.

### Changed

- **Tests are about 7 times faster and much lighter**: `scripts/test-all.sh` runs the whole battery in about 8 minutes (`--quick`: 1 minute) with three virtual machines of 1.5 GB at a time, instead of more than an hour one after another. Debug images boot silently (a verbose boot over the emulated 115200-baud serial port took two minutes per boot; now 20 seconds). WSL is capped (`%USERPROFILE%\.wslconfig`: 6 GB and automatic memory reclaim) so it no longer keeps 15 GB.
- Interface: the Rebootera logo and the version are now a simple footer of the side menu.
- The wizard has a new "Installer settings" step (answers, Windows 11 check, start only once) and the drivers step shows the drivers found in the USB.
- The catalog now says honestly which entries have a signed index (`signature`: Arch Linux and Alpine are `none`).
- `p7zip-full` and `efibootmgr` are part of the image.

### Performance

- **Idle CPU of the interface: from about 90 % to 4 %** in a virtual machine (window 47 % to 2 %, compositor 28 % to 1 %). Without a GPU the interface painted three endless pulse animations and every frame was composited in software. `start-kiosk` now tells the page when it is drawing in software (no `/dev/dri/renderD*`, or the software fallbacks) and the interface switches to a **light mode**: no endless animations, no blur or glow shadows, a short intro and slower polling. On computers with a real GPU nothing changes.
- **The interface starts earlier**: `rebootera-ui` no longer waits for `plymouth-quit-wait` (the end of the whole boot) because it already closes Plymouth itself just before opening its window; the boot splash halo is also redrawn at a third of the frame rate.
- Boot to a working interface in a 1 GB virtual machine: about 12 s (kernel 2 s + userspace 10 s). Memory in use after boot: about 560 MB.

### Fixed

- **The kernel log of the interface never worked**: Core asked `journalctl` for `--kernel`, an option that does not exist (it is `--dmesg`), so the widget stayed empty and the request failed every 10 s.
- **Black screen in VirtualBox** (VMSVGA graphics without 3D): Mesa chose the `vmwgfx` driver, EGL failed ("No 3D enabled") and `cage` aborted and then hung, so nothing restarted it. The interface is now started by `start-kiosk`, which tries hardware rendering, then Mesa software rendering (llvmpipe) and finally wlroots pixman with GTK cairo, and remembers the level that works. Checked in VirtualBox with VMSVGA (EFI and BIOS), VBoxSVGA (EFI) and VBoxVGA (BIOS).

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
  - Ventoy configured with `VTOY_SECURE_BOOT_POLICY=0` and Windows 11 requirement bypasses disabled (`VTOY_WIN11_BYPASS_CHECK=0`, `VTOY_WIN11_BYPASS_NRO=0`). Policy 1 does not let Linux start with Secure Boot, so policy 0 is the only one that works; the trade-off is that once the key is enrolled, the computer also starts images without a verified signature. The interface, the README and the limitations document warn about it.
- **Boot compatibility** of the official images from the Rebootera boot menu, checked in a virtual machine (`scripts/compat-matrix.sh`): shown in the Download Center, the library and the wizard, which does not let you pick the incompatible ones. Debian 13, Ubuntu 26.04 and Fedora 44 start (UEFI), Arch Linux starts in BIOS only (black screen in UEFI) and Alpine 3.24 does not start with Ventoy 1.1.18.
- **Driver note** in the Driver Assistant: the Windows installer needs the storage and network drivers on the USB drive (`drivers/`); injecting them into the installer is not done yet.
- `scripts/vmtest-interactive.py`: drive a VM from outside (keys, clicks, screenshots), used to walk through MokManager with Secure Boot.
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

- Boot menu (GRUB) fonts: the entries and the selection box now draw correctly with the bundled fonts.
- `vmtest-boot.py` did not drain the serial console, so QEMU stalled the guest and a debug image looked extremely slow.
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

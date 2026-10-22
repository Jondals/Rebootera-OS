# Changelog

All notable changes to Rebootera OS. Dates use the format YYYY-MM-DD.
Spanish version: [CHANGELOG.es.md](CHANGELOG.es.md).

## [0.20.0] - 2026-10-20

### Added

- **The small boot partition (VTOYEFI) no longer shows in Windows.** Creating or updating a USB from Rebootera Desktop removes its drive letter and asks Windows not to give it one again on that computer.
- **Rename partitions** (ext4, NTFS, exFAT and FAT): the name shows in Windows and Linux and the files are not touched.
- **A light browser that opens at once.** The Browser button no longer starts Firefox from the USB (it took ages): it opens a window in the same process as the interface, with back, forward, reload, an address bar that also searches, a loading bar and a close button. Nothing is saved (no cookies, no history) and it does not download files. Firefox, uBlock Origin and Dark Reader are gone from the image, which also makes it smaller.
- **Wi-Fi networks are remembered.** When you connect there is a "Remember this network on the USB" switch (on by default); at the next start Rebootera connects by itself as soon as the adapter is ready. Remembered networks are marked in the list and "Disconnect" forgets them. The password is stored in the Vault of your own USB.
- **No internet, no stuck download.** Pressing Download without a connection takes you to the Wi-Fi panel (already searching for networks) with a notice, instead of failing.
- **Downloads resume by themselves.** A download cut by the network is retried every few seconds from where it stopped, and one that was running when the computer was switched off continues at the next start. Windows links expire after about a day: Rebootera asks Microsoft for a new one and keeps the part already downloaded.
- **The window follows the theme in Rebootera Desktop:** the icon (title bar, taskbar, Alt+Tab) and the title bar color change with the theme, and so does the tab icon in any browser. The logo and the intro already did.
- **Partition changes are safer.** A copy of the partition table is saved before every change (in memory and in the USB Vault); if a step fails before any data is moved, the previous table is restored; and nothing is changed while a download is running.

### Changed

- **Order of the top bar.** Rebootera Desktop: Home, Create USB, Download, Drivers, My images, Settings. Rebootera OS: Home, Disks, Install, My images, Drivers, Download, This PC, Settings.
- **Pointer and keyboard layout apply at once, without restarting the screen** (before, every change closed and reopened the interface, and in some computers it did not come back). The pointer is applied by reloading the compositor settings; the keyboard has both layouts loaded and switches with a virtual Scroll Lock key.
- **Touchpad:** double tap no longer waits for a possible drag (it failed at times) and the scroll direction is set explicitly for touchpads.
- **Smoother on the USB.** Every section is read from the USB in the background once the interface is up, so switching is instant; while one is still being read a pulsing dot shows the tap was received; the Browser button says "Opening…". Without GPU the background is flat (no noise, vignette or blurred shadows), which is what cost most when scrolling or changing section.
- **Joining partitions is clearer and much faster.** Each choice says in one line which partition stays, how big it becomes, which one is erased, and whether it is quick (nothing is copied) or slow (and how long). When the one that stays is on the right, it is first shrunk to what it really uses and only that is copied, instead of the whole partition (that is what took 20 minutes).
- **Drivers:** more space between blocks and rows. A partition labelled DRIVERS is now always listed under "where drivers are searched", even if it only holds installers and no .inf packages (it says so).

### Fixed

- **Background blur did not work in Rebootera Desktop:** the light mode (always on there) turned off every SVG filter, including the one of the background. It is now stronger too (4, 10 and 22 px).
- Windows 10 and 11 (Home and Pro) download directly from Microsoft in the Download page (the entries were already in the code but not in the released build).

## [0.18.0] - 2026-10-20

### Changed

- **Installing is now three clear steps: System, Where, Install.** The "Where" step shows your disks drawn to scale and you pick a partition or a free space, not just a whole disk. It tells you at once whether the system fits and what will happen ("this partition will be formatted" or "nothing is erased"). If it does not fit, **Make room** lists the partitions that can be shrunk and opens the resize with the amount already set; the progress is shown in the same screen. The last step lists what to pick in the original installer ("pick the partition of 200 GB", "pick the Unallocated space of 120 GB") and a short plan; the old step with the hardware inspection and the driver checkbox is gone from the way: the drivers of this computer go to the Windows installer by themselves, a missing one is flagged with a button to the Drivers page, and everything technical (automatic answers, Windows 11 requirements, drivers, start only once) is inside "More options". If the computer does not meet Windows 11, one notice says so with a button to skip the check.
- **Disks, in plain words.** "Resize" is now "Bigger or smaller" and says that what you remove becomes free space; "Merge" is now "Join with a neighbor" and each choice shows the result ("one partition of 300 GB") and, in red, what is erased. The free space has "Create a partition here" and "Add the space to <the partition before it>", and the end message tells you to tap the free space to use it. The resize no longer talks about sharing the difference unless there is a neighbor, and says it as "Leave it as free space" or "Share it with <name>".
- **Drivers, rebuilt.** One answer at the top ("All set" or "Missing drivers: 1, without them Windows may not see the Wi-Fi") and, below, what matters for installing (network and disks), each piece with Ready (and where the driver was found) or Missing (with a button to search the web with the Rebootera browser and, folded, three steps to add it). Other devices and where drivers are searched are folded. In Rebootera Desktop the page explains in three steps how it works, shows a summary with a warning if a package has no valid signature, groups the drivers by folder with their type (network, storage...) and has a help text on which drivers are needed.
- **The browser is no longer hidden:** Rebootera OS has a **Browser** button in the top bar on every page (not only a card on Home).
- Shorter texts in all these pages, in English and Spanish.

### Fixed

- A partition that is open (mounted) could be offered for resizing and then fail; the button now says why it is disabled.

## [0.17.0] - 2026-10-20

### Added

- **Unraid** and **Synology DSM** (official-page entries: Unraid is made with its own USB creator and DSM only installs on Synology devices, and the cards say so) and **Proxmox Backup Server**.

### Fixed

- **Partitions that sometimes could not be changed.** The search for drivers left every partition with Windows or driver folders open read-only, and any open partition locked its whole disk ("close the open drives"); `ntfsresize` also refuses a partition in use. Rebootera's own read-only mounts no longer lock the disk and the agent closes them before measuring or changing anything. A partition labelled DRIVERS is also kept open for the driver search (before, it was closed again right away when its files were loose in the root).
- **Background blur works in Rebootera OS again** (the filter now goes on each moving piece of the background, so the GPU reuses it instead of filtering the whole screen every frame).
- **"This PC"**: the cards are aligned in a 2 by 2 grid and the long "other devices" list (that is the Drivers page's job) is gone; graphics are now in the processor card. The "About this computer" block of Home (duplicated) and the collapsible "More options" of Settings are gone: its settings are shown directly.

### Changed

- **Much faster to open, especially from a USB:** each page is now its own file read only when opened (the first screen loads about 140 KB of JavaScript instead of 220 KB; the rest comes with each page), the HTML preloads the files it needs, text is sent compressed and the files with a hash in their name are cached for good. Free space of the mounted partitions is measured in the background and cached (NTFS drives could take seconds in every read of the Disks page, which now says "Loading…" at once). Lighthouse (mobile profile, local server): performance 88 to 97, accessibility 96 to 100, best practices 100. The splash screen is skipped when a program drives the window.

## [0.16.0] - 2026-10-20

### Added

- **More systems in the Download Center** (all checked against their real sources, with size and SHA-256): **TrueNAS SCALE** (25.10 Goldeye and 25.04 Fangtooth), **XCP-ng** 8.3, **openmediavault**, **VyOS** (rolling), **Ubuntu Server cloud images** for VMs and VPS (24.04 and 22.04), the **Arduino IDE** for Windows, Linux and macOS (Intel and Apple silicon; the program to code Arduino boards, saved on the USB to install elsewhere, not an operating system), and **OPNsense** and **pfSense** as official-page entries (they ask for an image type or a form).
- A new `manifest` resolver for projects that publish a JSON with the latest version and keep the image and its SHA-256 in that version's folder (TrueNAS).
- The library also keeps programs (`.exe`, `.AppImage`, `.dmg`) and `.qcow2` disk images.

### Changed

- **Updating Rebootera now deletes the previous version** from the USB (and any old "previous" copy). The new copy is checked before it replaces the current one, so a cut in the middle never leaves the USB without a system. The "previous version" entry is gone from the boot menu.

## [0.15.1] - 2026-10-20

### Fixed

- **The chosen theme is saved and reaches Rebootera OS.** Rebootera Desktop lost it at every start (the browser storage belongs to a local address whose port changes each time) and, with no USB selected, nothing was written anywhere. Now the settings are also kept in a file on the PC; when a Rebootera USB is chosen the most recent ones win and go to the USB, so the system that starts from it already opens with that theme, its intro and the boot menu colors.
- **The logo and the intro animation follow the theme in the desktop app.** Chromium did not repaint the SVG gradients when only the theme variable changed; their colors are now set on the elements themselves at every change.
- The saved theme and language are written into the first page, so nothing flashes the default theme before the intro.

## [0.15.0] - 2026-10-20

### Added

- **Create USB now works on Linux and macOS too**, not only on Windows. On Linux it uses the official Ventoy package (downloaded and verified) with administrator permission asked through polkit, and mounts the new USB with udisks (or with a plain mount if udisks is missing). On macOS it builds Ventoy's layout with `diskutil` (GPT, exFAT "REBOOTERA" and the 32 MiB boot partition written from Ventoy's own image) and asks for the password with the system dialog; a USB made on a Mac starts UEFI computers. Only external USB disks that do not hold the system are offered. Adding Rebootera to a USB without erasing it stays Windows-only.
- `scripts/test/test-create-linux.sh` creates a USB on a virtual disk through the real app and checks it (real Ventoy install, label, GPT, boot partition, Rebootera OS byte for byte).

### Changed

- The interface is now one minified JavaScript file and one stylesheet instead of about 30 files (a read of the USB each): from about 750 KB to about 450 KB, and far fewer files to open at start-up (esbuild).
- A smaller image: no man pages, no docs apart from each package's licence text, and only the English and Spanish system languages.

## [0.14.0] - 2026-10-20

### Added

- New "This PC" tab with everything Rebootera found about the computer (firmware, processor, memory, graphics, disks, network and other devices with their driver). It replaces the kernel log.
- A missing network adapter driver is reported in Settings, with a button that searches the computer's disks.
- The partition named like "DRIVERS" (or similar) is read as a whole and wins over the drivers store of an installed Windows (shown as "used only when nothing else matches"); the Windows installer takes matching drivers by default.
- "Official page" opens the built-in browser (dark mode, no telemetry); the theme colors also reach the Ventoy menu on the USB.
- Touchpad support for I2C/HID laptop panels (modules loaded at boot).

### Changed

- Erasing or changing a drive no longer asks to type a phrase: a "Continue" button with the warning replaces it (also for partition changes).
- Wi-Fi is the first block in Settings; "Merge with a neighbor" is just "Merge".
- Partition errors say why (hibernated Windows, pending chkdsk, drive in use, no space, failing disk...) with the tool's last lines.
- Download sizes show instantly from a disk cache and refresh in the background without redrawing the cards.
- Performance: the animated background stays still by default and is never blurred while moving in Rebootera OS; more read-ahead for the USB. The saved theme and language are written into the first page, so the intro already uses them.
- Ventoy menu waits 1 s (it always shows its menu for at least that long).
- Less lag: the interface only redraws when something visible changed (not for the uptime or traffic counters), the USB is polled every 6 s (every 1.5 s only while a download runs), the system and sensors every 6 s, and WebKit uses no smooth scrolling or page cache (and no GPU in light mode). Automatic updates, periodic disk checks and the network wait are masked at boot.
- Clean-up: about 70 unused CSS rules, the temperature and traffic histories and the kernel log code are gone.

## [0.13.0] - 2026-10-20

### Changed

- The website (`site/`) is rebuilt with Astro and Tailwind: animated particle background that follows the theme, eight live theme cards (no dropdown, no button icons), bento feature grid, screenshot gallery, per-platform downloads and an FAQ. `scripts/dev/site.ps1` now builds it before serving, and the sync workflow publishes `site/dist`.
- `scripts/test/test-all.sh` is fast by default (under 3 minutes: both ISOs on UEFI + Secure Boot, rebuilding only when the source changed); the full battery is `--full`. The release-ISO test clicked at old header coordinates and failed to power off: fixed.

## [0.12.0] - 2026-10-20

### Changed

- **Rebootera OS now starts straight away.** The live image no longer shows its boot menu or waits: it goes directly into the interface (hold Shift at power-on on BIOS computers to see the menu with "safe graphics").
- **The graphical session no longer gets stuck on a black screen.** The interface now reports when it has drawn its first frame, and the start-up only trusts a rendering level once that happens. If the window crashes or stays black it tries the next level (hardware, software, plain software) and, as a last resort, goes back to the simpler `cage` compositor instead of repeating the window over and over.
- **A completely new logo:** a sunrise (a sun rising over the horizon under two signal arcs: the "re" of restart and the "era" of a new start) on a dark rounded tile, recoloured by each theme, and new icons for the app, the boot menu and Rebootera Desktop.
- **A new intro instead of a loading bar:** it dawns: the horizon lines open, the sun rises from behind them, the arcs are drawn and the name enters letter by letter; it is skippable and turns into a plain fade on modest computers.
- **No side menu any more: navigation is a header with tabs.** The brand is on the left, the sections are tabs in the middle (with their name, or just the icon on narrower windows) and on the right go the USB with its free space, the theme, the language and power. All the width is for the content, and Settings is the last tab.
- **Themes: all dark, Amoled first and by default.** The light theme (Ivory) is gone (anyone who had it now gets Amoled) and a new **Orange** theme joins Obsidian, Sapphire, Emerald, Amethyst, Ember and Graphite.
- **Drop-downs animate:** the collapsible sections and the version list open and close smoothly instead of jumping.
- **"Make USB" opens at once:** the window used to stay empty for seconds while Windows listed the disks. Now the page appears immediately ("Looking for USB drives…"), the disks are read in the background from the moment the app starts and are shown as soon as they arrive.
- **The website follows the app's themes** (the same eight dark themes, with a theme picker and the logo recoloured) and can be viewed with `scripts/dev/site.ps1`.
- **Theme and language now live only in Settings** (cards with a preview for each theme, and the language choice), not in a drop-down in the header.
- **The header sits at the very top** (a hidden 0-size icon sheet was taking a blank line above it), the download cards are all the same size with the buttons at the bottom, and the "Official page" button is short.
- **The intro plays in full in Rebootera Desktop** (it used to be cut to a third of a second because the app counts as "light" for the background).
- **More versions:** Ubuntu 25.10, Debian 11 and every Debian 12 live flavour, Fedora 42 and the 43 spins, Linux Mint 21.3, Alpine 3.22 and 3.21, Rocky and AlmaLinux 8, NixOS 25.11 and 25.05, CentOS Stream 9, FreeBSD 14.4, OpenBSD 7.8 and 7.7, Proxmox VE 8 and Windows Server 2019 and 2016 (evaluation).
- **Changing the language no longer reloads everything:** the page keeps its place and its entrance animation is not repeated.
- **Make USB shows its progress at once** when you come back to it, instead of waiting for the disks to be read again.
- **Settings buttons now stay selected** when you press them (background, blur, size, menu).
- **`out/` only holds the ISO and the .exe.** The Linux and macOS binaries go to `dist/`, used by the automatic publication.

### Fixed

- **Black screen with only the mouse pointer, and screen flashes.** Two causes. Core applied "saved" pointer settings that did not exist (all zero: speed 0, tap to click off) at every start, which force-restarted the interface a second after it opened and left the touchpad without tap to click. And when the session was restarted, the old compositor survived (systemd moves it out of the service group) and kept the screen, so the new one could not draw. Both are fixed, with tests.

### Added

- **Six more themes, all dark:** Sapphire stays, and Emerald, Amethyst, Ember, Graphite and Amoled join Obsidian. The theme menu shows a miniature of each one.
- **Configurable background blur** (none, soft, medium, strong) to calm the animated background; it is switched off by itself on modest computers.
- **Boards & IoT tab** with Raspberry Pi OS (six editions), Ubuntu for Raspberry Pi (26.04 and 24.04, server and desktop), DietPi (Raspberry Pi 1 to 5, ODROID-N2 and virtual machines), OpenWrt (PC and Raspberry Pi 4) and Home Assistant OS (Raspberry Pi 3/4/5, Green and generic). They are checked with their published SHA-256. Arduino is not an operating system, so it is not in the list.
- **More versions of existing systems:** Ubuntu 22.04 (desktop and server), Debian 12 (netinst and live GNOME, KDE and Xfce), Fedora 43, Linux Mint LMDE 7, AlmaLinux 9 and Windows 10 LTSC 2021 in English.
- **Name and picture for your images:** "Customize" in My images lets you rename an image and give it a picture (resized to 128 px; the file is not touched). The installation wizard shows the name you chose.
- A new kind of source ("latest + checksum") for images whose stable link redirects to the newest file and publishes its SHA-256 next to it.

## [0.11.0] - 2026-10-19

### Changed

- **New logo and icons.** The mark is now an "R" inside a boot-arrow ring in brass, drawn once and recoloured by each theme. Every icon of the interface was redrawn (the disk icons included) and the boot menu, splash and Rebootera Desktop icon use the same mark.
- **New boot menu (UEFI/GRUB and Ventoy).** Rebuilt theme with the logo, rounded 9-slice selection bars, a clear title and key hints; the live ISO boots straight into the kiosk.
- **Home is a launcher.** One search bar and a short list of actions with keyboard shortcuts (arrows, Enter, numbers); typing a system name offers to download it. It replaces the card grid.
- **Square buttons.** Buttons, chips and badges use an 8–11 px radius instead of pills.
- **Theme switcher with real previews.** The top bar shows a small preview of the current theme and opens a list with a miniature window of each one; Settings uses the same miniatures.
- **Download: a real version picker** instead of the browser's drop-down list (edition, size and a "Recommended" mark).
- **Make USB (Rebootera Desktop) simplified.** Each USB appears once. Pick it and choose what to do: update Rebootera (if it already has it), add Rebootera and keep your files, or erase and start clean. Rebootera comes from the latest version (downloaded and checked) or from your own Rebootera ISO. Adding images and driver folders moved out of this screen.
- **Partition manager simplified** to three actions: create/resize, merge into a neighbour, delete. The duplicate "share space" options are gone; the resize dialog now says what it does. Partition names such as `basic\x20data\x20partition` are shown as "Data" and NTFS limits (hibernation, Fast Startup, BitLocker, dirty volume) are explained instead of failing silently.
- **Smoother and lighter.** Page transitions with a staggered entrance, an animated background with more movement (orbits and sweeping light beams, transform-only, not blobs), polling that stops while the window is hidden and runs less often on modest computers, and an automatic "still background" on computers with 4 cores or less. About 20 % of the stylesheet, unused code and unused texts were removed.
- **"Windows from Rufus" became "Installation media".** Any installation made with any tool (Rufus, Ventoy, a folder, a plain ISO) can be added.
- **Project layout.** Scripts are now grouped in `scripts/build`, `scripts/usb`, `scripts/test` and `scripts/dev`; the kiosk window moved to `os/shell`. The plan and limitations documents were removed; what they described is built or listed in the README.

### Added

- **Drivers are found by themselves.** At every start Rebootera looks for Windows driver packages on the USB, in driver folders of this computer's partitions (the ones manufacturers leave, such as a DRIVERS partition or SWSetup) and in an installed Windows, all read-only. The Drivers page shows where each package comes from and has "Search again".
- **Pointer settings in the live system:** pointer speed, tap to click (on by default; the touchpad needed a hard press before) and natural scrolling, saved on the USB.
- **A browser inside Rebootera OS:** Firefox, only to browse, with uBlock Origin and Dark Reader installed and locked, no extensions store, no sync and no telemetry. The kiosk now runs on labwc (a light compositor).
- **Download from a link:** paste an https link to an image, or an Internet Archive item, and pick the file; it is saved to the USB as unchecked (SHA-1 is verified when the Archive publishes it).
- **More customisation:** animated background (automatic, always, never), interface size (90–130 %), pointer settings, theme, language, keyboard and Wi-Fi.
- **Rebootera Desktop for Linux and macOS** (x64 and arm64, a single binary each). It opens in its own window when a Chromium-based browser is installed, uses zenity/kdialog (Linux) or AppleScript (macOS) for file dialogs and detects mounted drives. Creating the USB from the app is still Windows-only (on Linux use `scripts/usb/make-usb.sh`); making an ISO from a folder is not available on macOS.
- Windows privacy option: `ReduceTelemetry` is now optional per installation and language blocks are optional.

### Fixed

- Rebootera Desktop for Windows no longer shows the address at the bottom left when hovering the menu.
- The partition manager could refuse to resize the base disk/partition; the reasons are now explicit and resizing works on supported file systems.

## [0.10.0] - 2026-10-19

### Changed

- **A new look, from scratch.** Dark, elegant and calm: warm black with brass accents, serif titles (Fraunces) and Inter for text, hairline borders instead of heavy shadows, a compact icon sidebar and a background of slow-moving lights with a few specks of dust (it only moves layers, so it costs almost nothing). Fonts are bundled, so it looks the same offline.
- **Three themes, remembered** (also on the USB): Obsidian (dark, brass), Ivory (light, like paper) and Sapphire (deep blue). Switch from the three dots in the top bar or from Settings.
- **Every system has its logo** in Download (Windows, Ubuntu, Debian, Fedora, Mint, Arch, openSUSE, Kali, Rocky, Alma, Proxmox, FreeBSD, OpenBSD, Alpine, NixOS, Void, CentOS and more; the rest get a monogram).
- **Downloads only go to your Rebootera USB, and it says so.** A banner at the top of Download shows which USB they are saved to and how much space is left; without a USB it asks you to connect one. The "library on this PC" is gone: nothing is ever downloaded to the computer's disk. Rebootera Desktop picks your Rebootera USB by itself as soon as it is connected.
- **Home** has fewer, bigger cards, now including "Windows from Rufus".
- **Disks and partitions** redone: each part shows how full it is, with a colour legend; picking a part shows its summary and clear action cards (Resize, Share space with a neighbor, Merge, Delete, Create) with one line each and the reason when one is not possible.
- **The intro is light**: no blurred layers or blend modes (that is what made it stutter), only a fade and a thin bar; about 1.2 s minimum, and almost instant on computers without graphics acceleration.

### Added

- **Reduce Windows telemetry** (Install, Options, for Windows images): while installing it sets the diagnostic-data policies to zero, disables the DiagTrack and dmwappushservice services, turns off advertising and suggested content and skips the express-settings screen, with no change to your language or disks. It works fully on Enterprise, Education and LTSC (so Windows 11 LTSC); on Home and Pro Windows keeps its basic level.
- **Windows made with Rufus is easier to add**: "Windows from Rufus" on Home and the top of My images. Plug in the USB Rufus made (or choose its folder) and it becomes a bootable ISO on your Rebootera USB, keeping everything Rufus added (answer files, privacy settings).
- **Internet Archive** as a source: FreeDOS 1.2, Haiku R1/beta4, Tiny Core Linux (CorePlus) and KolibriOS, with the SHA-1 the Archive publishes checked after downloading (it detects damaged copies, not forged ones; the page says so). New "Other" tab for them.

### Fixed

- Rebootera Desktop no longer fills the PC's disk with a hidden local library.

## [0.9.0] - 2026-10-19

### Changed

- **New look: Frutiger Aero.** Glass panels, glossy buttons, a sky with a green hill and bubbles that rise slowly (they only move layers, so they cost almost nothing), a compact icon sidebar and smaller, tighter pages. The whole interface was restyled.
- **Three themes, remembered.** Sky (light), Night (dark glass) and Forest (light, green). Pick one from the three dots in the top bar or from Settings; the choice is saved (and on the USB, so Rebootera OS starts with it next time).
- **Settings** now starts with the theme cards; accent color is gone (the themes replace it).
- **Rebootera Desktop** keeps the animated background (it used to be switched off there to save CPU); blur and glow stay off.

### Fixed

- **A new USB made from Windows now uses an MBR partition table** (Ventoy's default), which starts on old BIOS PCs and on UEFI. Before it was always GPT, and some computers' boot menus do not list a GPT-only USB. GPT is only used for disks over 2 TiB.
- Catalog links with absolute paths (some Fedora mirrors) no longer break the checksum lookup.

### Added

- **25 more systems** (65 in 24 families): Ubuntu Budgie, Cinnamon, Unity, Edubuntu and Studio; Debian Live Cinnamon, MATE, LXQt, LXDE and Standard; Fedora Budgie, LXQt, COSMIC, Sway, Silverblue and Kinoite; NixOS 26.05 (minimal and graphical); Void Linux (base and Xfce); CentOS Stream 10 (boot and DVD); Proxmox Backup Server and Mail Gateway; Alpine Extended. Every new entry was resolved against the real servers; where the project signs its checksums the signature is verified.

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

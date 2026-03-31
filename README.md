# TekstoBot 🤖
[![FOSSA Status](https://app.fossa.com/api/projects/git%2Bgithub.com%2Frda-run%2Ftekstobot.svg?type=shield)](https://app.fossa.com/projects/git%2Bgithub.com%2Frda-run%2Ftekstobot?ref=badge_shield)


**TekstoBot** is a WhatsApp processing service that utilizes local Artificial
Intelligence to automatically transcribe speech to text (**STT**).

## 🚀 Features

- **Audio Transcription:** Receive voice messages and get the corresponding text
  via Whisper.
- **Administrative Dashboard:** Web interface to manage authorized numbers, view
  history, and monitor connection status.
- **Whitelist:** Only authorized numbers in the database can interact with the
  bot.
- **Asynchronous Processing:** Uses goroutines and worker pools to ensure the
  bot remains responsive.

## 🛠️ Tech Stack

- **Language:** [Go (Golang)](https://go.dev/)
- **Database:** [PostgreSQL](https://www.postgresql.org/)
- **WhatsApp Integration:** [whatsmeow](https://github.com/tulir/whatsmeow)
- **AI (STT):**
  [faster-whisper-server](https://github.com/fedirz/faster-whisper-server) (via
  Podman/Docker)
- **Frontend:** Go `html/template`, [HTMX](https://htmx.org/) and [Tailwind
  CSS](https://tailwindcss.com/)

## 📋 Prerequisites

1. **Go 1.21+** installed.
2. **PostgreSQL** running locally.
3. **Podman** (or Docker) for the Whisper server.
4. **psql** (PostgreSQL client) to run migrations via Makefile.

### 🔧 Prepare GPU Acceleration (Additional for Podman on AlmaLinux/RHEL)

For the voice transcription (`make whisper`) to natively access your GPU through
Podman using CDI mapping:

1. **Add the official NVIDIA repository**:

   ```bash
   curl -s -L https://nvidia.github.io/libnvidia-container/stable/rpm/nvidia-container-toolkit.repo | \
     sudo tee /etc/yum.repos.d/nvidia-container-toolkit.repo
   ```

2. **Install the Toolkit**:

   ```bash
   sudo dnf install -y nvidia-container-toolkit
   ```

3. **Generate Hardware Descriptors (CDI)** so Podman recognizes the environment:

   ```bash
   sudo mkdir -p /etc/cdi
   sudo nvidia-ctk cdi generate --output=/etc/cdi/nvidia.yaml
   ```

## ⚙️ Configuration

1. Clone the repository:

   ```bash
   git clone <repo-url>
   cd tekstobot
   ```

2. Configure the environment variables:

   ```bash
   cp .env.example .env
   # Edit the .env file with your database credentials
   ```

3. Install dependencies:

   ```bash
   go mod tidy
   ```

## 🏃 How to Run

The project uses a `Makefile` to simplify common commands:

1. **Run Migrations:**

   ```bash
   make migrate-up
   ```

2. **Start Whisper (Audio AI):**

   ```bash
   make whisper
   ```

3. **Run the Bot:**

   ```bash
   make run
   ```

4. **Access the Dashboard:** Open your browser at `http://localhost:8080` (or
   the port defined in your `.env`).

## 📖 Makefile Commands

Run `make` to see the list of available commands:

- `make build`: Build the binary.
- `make release`: Build the optimized binary for production.
- `make package`: Generate the RPM package using nfpm.
- `make run`: Run the project locally.
- `make whisper`: Start the Whisper container via Podman.
- `make whisper-stop`: Stop and remove the Whisper container.
- `make migrate-up`: Run up migrations.
- `make migrate-down`: Rollback migrations.
- `make check`: Run build and import checks.

## 📦 Production Deployment (RPM + Quadlets)

For production environments on AlmaLinux/RHEL, TekstoBot can be installed as an
RPM package that automatically manages the Whisper and Bot containers via
**Podman Quadlets** and **systemd**.

### 1. Requirements for Building the Package

Ensure you have the `nfpm` utility installed to generate the package:

```bash
# Example nfpm installation (Go-based tool)
go install github.com/goreleaser/nfpm/v2/cmd/nfpm@latest
```

### 2. Generate and Install the Package

```bash
# Build the optimized binary and generate the RPM
make package

# Install the generated package
sudo dnf install ./tekstobot.rpm
```

### 3. Automatic GPU Configuration

The RPM post-install script detects if NVIDIA support is present on the host.

- **With GPU:** Whisper starts using the `cuda` image and CDI mapping.
- **Without GPU:** Whisper automatically falls back to `cpu` mode.

### 4. Service Initialization

Services are managed by systemd:

```bash
# Configure your .env (required before starting)
sudo cp /etc/tekstobot.env.example /etc/tekstobot.env
sudo vi /etc/tekstobot.env

# Start the services (the bot will automatically start Whisper if needed)
sudo systemctl enable --now tekstobot

# Follow the logs
sudo journalctl -u tekstobot -f
```

## 📄 License

This project is under the MIT license. See the [LICENSE](LICENSE) file for
details.


[![FOSSA Status](https://app.fossa.com/api/projects/git%2Bgithub.com%2Frda-run%2Ftekstobot.svg?type=large)](https://app.fossa.com/projects/git%2Bgithub.com%2Frda-run%2Ftekstobot?ref=badge_large)
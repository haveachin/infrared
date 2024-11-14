# Getting Started

There are multiple ways to install or setup you Infrared instance. Depending on what setup you choose you will have different pros and cons in usage and update cycles.

## Quick Start

One of the quickest ways to get started is by just downloading the [latest release of Infrared](https://github.com/haveachin/infrared/releases/) from GitHub for your machine and executing it.

### Find the binary for your system

Most common ones are in **bold**.

| Hardware                    | OS              | for 32-bit    | for 64-bit         |
|-----------------------------|-----------------|---------------|--------------------|
| PC, VPS or Root Server      | Linux based     | Linux_i386*   | **Linux_x86_64**   |
| Raspberry Pi                | Raspberry Pi OS | Linux_armv6*  | **Linux_arm64**    |
| Custom/Prebuilt PC          | Windows         | Windows_i386* | **Windows_x86_64** |
| Intel Mac or MacBook        | macOS           | -             | Darwin_x86_64      |
| M1 or higher Mac or MacBook | macOS           | -             | Darwin_arm64       |

\*These architectures are most of the time the correct, but there is more to it.

### Downloading

If your system has a desktop environment, then you should be able to download your binary by just clicking on the version you want on the releases page.
The URL of your download should look something like this:
```
https://github.com/haveachin/infrared/releases/download/{version}/infrared_{architecture}.tar.gz
```
For example:
```
https://github.com/haveachin/infrared/releases/download/v1.3.4/infrared_Linux_x86_64.tar.gz
```

::: tip
If you are using SSH to connect to a remote server and are currently using a desktop environment with a browser you can just right-click the version you need and copy the link. Then, paste it into your terminal with Ctrl+Shift+V on GNU/Linux or right-click on Windows.
:::

Downloading by using the terminal on macOS or GNU/Linux:
```bash
curl -LO https://github.com/haveachin/infrared/releases/download/{version}/infrared_{architecture}.tar.gz
```

Downloading by using Powershell on Windows:
```Powershell
Invoke-WebRequest -Uri https://github.com/haveachin/infrared/releases/download/v1.3.4/infrared_Windows_x86_64.zip -OutFile c:\infrared.zip
```

### Extracting the binary

Extracting by using the terminal on macOS or GNU/Linux:
```bash
tar -xzf infrared_{architecture}.tar.gz
```

Extracting by using Powershell on Windows:
```Powershell
Expand-Archive c:\infrared.zip -DestinationPath c:\
```
## Docker

[Official Image on Docker Hub](https://hub.docker.com/r/haveachin/infrared)  
[Github Registry](https://github.com/haveachin/infrared/pkgs/container/infrared)

### Docker Compose

```docker
version: "3.8"

services:
  infrared:
    image: haveachin/infrared:latest
    container_name: infrared
    restart: always
    ports:
      - 25565:25565/tcp
    volumes:
      - ./data/infrared:/etc/infrared
```

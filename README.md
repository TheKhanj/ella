# ella

`ella` is a simple process manager for running and managing services.

## Installation

- **Arch Linux (AUR):** available as [`ella`](https://aur.archlinux.org/packages/ella) and [`ella-bin`](https://aur.archlinux.org/packages/ella-bin)
- **macOS (Homebrew):** `brew install thekhanj/ella/ella`
- **Other platforms:** download from [GitHub releases](https://github.com/thekhanj/ella/releases)

## Build And Install

Run the following commands:

```sh
make
sudo make install
```

## Usage

Check help or man page:

```sh
ella -h
man ella
```

## Run ella:

```sh
ella run [-c ella.json] [starting-services...]
```

### Example Config (ella.json):
```json
{
  "$schema": "...",
  "services": [
    {
      "name": "service1",
      "process": {
        "exec": "sh -c 'for i in $(seq 5); do echo \"service1 running $i\"; sleep 1; done'"
      }
    }
  ]
}
```

## Manage services:

```sh
ella start [-c ella.json] [services...]
ella stop [-c ella.json] [services...]
ella restart [-c ella.json] [services...]
ella reload [-c ella.json] [services...]
```

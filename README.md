{{- /* Ignore this text, until templating is ran via [sensu-plugin-tool](https://github.com/sensu-community/sensu-plugin-tool) the below badge links wiill not render */ -}}

[![Sensu Bonsai Asset](https://img.shields.io/badge/Bonsai-Download%20Me-brightgreen.svg?colorB=89C967&logo=sensu)](https://bonsai.sensu.io/assets/sardinasystems/sensu-go-chrony-check)
![Go Test](https://github.com/sardinasystems/sensu-go-chrony-check/workflows/Go%20Test/badge.svg)
![goreleaser](https://github.com/sardinasystems/sensu-go-chrony-check/workflows/goreleaser/badge.svg)

# sensu-go-chrony-check

## Table of Contents
- [Overview](#overview)
- [Files](#files)
- [Usage examples](#usage-examples)
- [Configuration](#configuration)
  - [Asset registration](#asset-registration)
  - [Check definition](#check-definition)
- [Installation from source](#installation-from-source)
- [Additional notes](#additional-notes)
- [Contributing](#contributing)

## Overview

The sensu-go-chrony-check is a [Sensu Check][6] that ensures that local cronyd is having time source.

Unlike [sensu-plugins-chrony][11] uses unix socket directly. Solves #sensu-plugins/sensu-plugins-chrony/17 .


## Files

- sensu-go-chrony-check

## Usage examples

## Configuration

### Asset registration

[Sensu Assets][10] are the best way to make use of this plugin. If you're not using an asset, please
consider doing so! If you're using sensuctl 5.13 with Sensu Backend 5.13 or later, you can use the
following command to add the asset:

```
sensuctl asset add sardinasystems/sensu-go-chrony-check
```

If you're using an earlier version of sensuctl, you can find the asset on the [Bonsai Asset Index][https://bonsai.sensu.io/assets/sardinasystems/sensu-go-chrony-check].

### Check definition

```yml
---
type: CheckConfig
api_version: core/v2
metadata:
  name: sensu-go-chrony-check
  namespace: default
spec:
  command: sensu-go-chrony-check
  subscriptions:
  - system
  runtime_assets:
  - sardinasystems/sensu-go-chrony-check
```

## Installation from source

The preferred way of installing and deploying this plugin is to use it as an Asset. If you would
like to compile and install the plugin from source or contribute to it, download the latest version
or create an executable script from this source.

From the local path of the sensu-go-chrony-check repository:

```
go build
```

## Additional notes

## Contributing

For more information about contributing to this plugin, see [Contributing][1].

[1]: https://github.com/sensu/sensu-go/blob/master/CONTRIBUTING.md
[2]: https://github.com/sensu-community/sensu-plugin-sdk
[3]: https://github.com/sensu-plugins/community/blob/master/PLUGIN_STYLEGUIDE.md
[4]: https://github.com/sensu-community/check-plugin-template/blob/master/.github/workflows/release.yml
[5]: https://github.com/sensu-community/check-plugin-template/actions
[6]: https://docs.sensu.io/sensu-go/latest/reference/checks/
[7]: https://github.com/sensu-community/check-plugin-template/blob/master/main.go
[8]: https://bonsai.sensu.io/
[9]: https://github.com/sensu-community/sensu-plugin-tool
[10]: https://docs.sensu.io/sensu-go/latest/reference/assets/
[11]: https://github.com/sensu-plugins/sensu-plugins-chrony

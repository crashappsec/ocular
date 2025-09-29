<br />
<div align="center">
    <h1 align="center">
        <img alt="Ocular" src=".github/assets/img/logo.png"></img>
    </h1>
    
  <p align="center">
        Ocular extends Kubernetes to provide static scanning configuration that enables you to perform regular or ad-hoc security scans over static software assets.
        It provides a set of custom resource definitions that allow you to configure and run security or compliance scanning tools.
  </p>
</div>

<hr/>

[![Documentation Site](https://img.shields.io/badge/docs-ocularproject.io-blue)](https://ocularproject.io/docs/)
[![Artifact Hub](https://img.shields.io/endpoint?url=https://artifacthub.io/badge/repository/ocular)](https://artifacthub.io/packages/helm/crashoverride-helm-charts/ocular)
[![Go Reference](https://pkg.go.dev/badge/github.com/crashappsec/ocular.svg)](https://pkg.go.dev/github.com/crashappsec/ocular)
[![Go Report Card](https://goreportcard.com/badge/github.com/crashappsec/ocular)](https://goreportcard.com/report/github.com/crashappsec/ocular)
[![GitHub Release](https://img.shields.io/github/v/release/crashappsec/ocular)](https://github.com/crashappsec/ocular/releases)
[![License: GPL v3](https://img.shields.io/badge/License-GPLv3-blue.svg)](https://www.gnu.org/licenses/gpl-3.0)



## Overview

Ocular is a Kubernetes API extension that allows you to perform security scans on static software assets.
It provides a set of custom resource definitions that allow you to configure and run security or compliance scanning tools over static software assets,
such as git repositories, container images, or any static content that can be represented on a file system.

It is designed to allow for both regular scans on a scheduled basis or, ad-hoc security scans ran on demand.
The system allows for the user to customize not only the scanning tools that are used, but also:
- How scan targets are enumerated (e.g. git repositories, container images, etc.)
- How those scan targets are downloaded into the scanning environment (e.g. git clone, container pull, etc.)
- How the scanning tools are configured and run (e.g. custom command line arguments, environment variables, etc.)
- Where the results are sent (e.g. to a database, to a file, to a cloud storage etc.)

Each of these components can be configured independently, allowing for a high degree of flexibility and customization.
Each of the 4 components (enumeration, download, scanning, and results) can be customized via a container image that implements a specific interface,
normally through environment variables, command line arguments and file mounts.

For more information on Ocular and how to use it, see the [Ocular project site](https://ocularproject.io/docs/).

## Getting started

### Installation via Helm

See the [installation guide](https://ocularproject.io/docs/getting-started/install) on our documentation site for instructions on how to install Ocular via Helm.

### Running locally

See [DEVELOPMENT.md](docs/DEVELOPMENT.md) for instructions on how to run the application locally.

## Contact

We are constantly learning about emerging use cases and are always interested in hearing about how you use Ocular.
If you would like to talk, [please get in touch](https://ocularproject.io/contact).



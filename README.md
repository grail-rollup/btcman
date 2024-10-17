<div id="top"></div>
<!-- PROJECT LOGO -->
<br />
<div align="center">

<img src="./.github/assets/btcman_logo.png#gh-light-mode-only" alt="Logo" width="100">
<img src="./.github/assets/btcman_logo.png#gh-dark-mode-only" alt="Logo" width="100">

# **btcman**

</div>
</div>

`btcman` is a Go-based Bitcoin transaction manager. It simplifies the interaction with the Bitcoin blockchain, allowing the inscription of data in Bitcoin transactions. The project utilizes key libraries from the Bitcoin ecosystem, such as `btcsuite`, and integrates with the Polygon CDK for extended functionality.

## Info

- Follows the ordinals format for writing data to the BTC network
- Connects to an Electrum server indexer in order to comunicate with the BTC network
- Consolidates UTXOs in order to reuse them for later transactions

## Installation

```bash
go get github.com/grail-rollup/btcman
```

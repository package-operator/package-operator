Images:
  package-operator-manager: "##pko-manager-image##"
  package-operator-package: "##pko-package-image##"

# `Config` gives the installing user direct access to the package-operator package's configuration key.
# It first gets passed into bootstrap job as envvar PKO_CONFIG.
# Then the bootstrap job creates ClusterPackage/package-operator and supplies given data into `.spec.config`, which is PKO's official configuration API.
# Look at `config/packages/package-operator/manifest.yaml.tpl` in the pko upstream repo to discover available configuration options.
Config: {}

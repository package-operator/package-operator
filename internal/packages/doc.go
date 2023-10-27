// The packages package contains a library to import, parse, validate and deploy Package Operator packages.
// Files/OCI/etc. are imported as RawPackages, containing a Package filesystem.
// RawPackages can be loaded as Package, containing the package filesystem and parsed PackageManifests.
// Packages and a PackageRenderContext can be rendered into a PackageInstance, ready to be deployed.
package packages

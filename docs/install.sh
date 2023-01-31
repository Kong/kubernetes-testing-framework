#!/usr/bin/env bash

set -euo pipefail

BASE="kong/kubernetes-testing-framework"

# ------------------------------------------------------------------------------
# Validate OS Type
# ------------------------------------------------------------------------------

echo "INFO: verifying operating system compatibility"

OSTYPE=$(uname -s | tr '[:upper:]' '[:lower:]')

if [ "$OSTYPE" != "linux" ]; then
    if [ "$OSTYPE" != "darwin" ]; then
        echo "Error: unsupported operating system ${OSTYPE}"
        exit 1
    fi
fi

# ------------------------------------------------------------------------------
# Validate Architecture
# ------------------------------------------------------------------------------

echo "INFO: verifying cpu architecture compatibility"

ARCH=$(uname -m | tr '[:upper:]' '[:lower:]')

if [ "$ARCH" = x86_64 ]; then
    ARCH="amd64"
fi
if [ "$ARCH" = aarch64 ]; then
    ARCH="arm64"
fi

if [[ "$ARCH" != "amd64" && "$ARCH" != "arm64" ]]; then
    echo "Error: ${ARCH} is not a supported architecture at this time."
    exit 1
fi

TEMPDIR=$(mktemp -d)

function download_with_curl()
{
    echo "INFO: downloading ktf via curl"

    # ------------------------------------------------------------------------------
    # Determine Latest Release
    # ------------------------------------------------------------------------------

    LATEST_RELEASE=$(curl -s https://api.github.com/repos/${BASE}/releases/latest | perl -ne 'print $1 if m{"name": "(v[0-9]+\.[0-9]+\.[0-9]+.*)"}')

    if [ "$LATEST_RELEASE" = "" ]; then
        echo "Error: could not find latest release for ${BASE}!${LATEST_RELEASE}"
        exit 1
    fi

    echo "INFO: the latest release of ${BASE} was determined to be ${LATEST_RELEASE}"

    # ------------------------------------------------------------------------------
    # Downloading Binary & Checksums
    # ------------------------------------------------------------------------------

    DOWNLOAD_URL="https://github.com/${BASE}/releases/download/${LATEST_RELEASE}/ktf.${OSTYPE}.${ARCH}"
    DOWNLOAD_CHECKSUMS_URL="https://github.com/${BASE}/releases/download/${LATEST_RELEASE}/CHECKSUMS"

    echo "INFO: downloading ktf cli for ${OSTYPE}/${ARCH}"
    curl -L --proto '=https' --tlsv1.2 -sSf ${DOWNLOAD_URL} > ${TEMPDIR}/ktf.${OSTYPE}.${ARCH}

    echo "INFO: downloading checksums for release ${LATEST_RELEASE}"
    curl -L --proto '=https' --tlsv1.2 -sSf ${DOWNLOAD_CHECKSUMS_URL} > ${TEMPDIR}/CHECKSUMS
}

function download_with_gh()
{
    echo "INFO: Downloading ktf via gh"
    gh release download --dir ${TEMPDIR} --pattern "ktf.${OSTYPE}.${ARCH}" --repo "${BASE}"
    gh release download --dir ${TEMPDIR} --pattern "CHECKSUMS" --repo "${BASE}"
    LATEST_RELEASE=$(gh release view --repo "${BASE}" --json tagName --template {{.tagName}})
}

if gh --version 2>/dev/null 1>/dev/null; then
    download_with_gh
else
    echo "INFO: gh not available on the system, falling back to download via curl"
    download_with_curl
fi

# ------------------------------------------------------------------------------
# Checksum Verification
# ------------------------------------------------------------------------------

pushd ${TEMPDIR} 1>/dev/null
sha256sum -c CHECKSUMS --ignore-missing 1>/dev/null
popd 1>/dev/null

# ------------------------------------------------------------------------------
# Installation
# ------------------------------------------------------------------------------

INSTALL_DIRECTORY="${HOME}/.local/bin" # TODO: will make this dynamic in a later iteration
[[ -d "${INSTALL_DIRECTORY}" ]] || mkdir -p "${INSTALL_DIRECTORY}"
INSTALL_LOCATION="${INSTALL_DIRECTORY}/ktf"

install ${TEMPDIR}/ktf.${OSTYPE}.${ARCH} ${INSTALL_LOCATION}
chmod +x ${INSTALL_LOCATION}

# ------------------------------------------------------------------------------
# Cleanup
# ------------------------------------------------------------------------------

rm -f ${TEMPDIR}/ktf.${OSTYPE}.${ARCH}
rm -f ${TEMPDIR}/CHECKSUMS
rmdir ${TEMPDIR}

echo "SUCCESS! Checksums verified, ktf (version: ${LATEST_RELEASE}) was installed at: ${INSTALL_LOCATION}"

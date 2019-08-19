# This Dockerfile is to build Windows containers running the driver.
# Run make Windows to use this Dockerfile.
FROM mcr.microsoft.com/windows/servercore:1809 as core

FROM mcr.microsoft.com/windows/nanoserver:1809

COPY ./bin/gcfs-csi-driver.exe /gcfs-csi-driver.exe
COPY --from=core /Windows/System32/netapi32.dll /Windows/System32/netapi32.dll
USER ContainerAdministrator
ENTRYPOINT ["/gcfs-csi-driver.exe"]
CMD []
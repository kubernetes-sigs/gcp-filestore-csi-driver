from pyNfsClient import (Portmap, RPC, Mount)
import socket

host = "10.129.116.194" # Filestore instance Ip
clientNodeIp = "10.128.0.152" # GKE Node Ip for gke-nfs-rpc-poc-default-pool-a74d3aa6-1rgg
timeout = 60 # seconds


NLM_PROG = 100021
NLM_VERS = 4
CUSTOM_NLM_PROGRAM = 200002
CUSTOM_NLM_PROG_VERS = 1
CUSTOM_NLM_PROC_VERS = 1

def ReleaseLocks():
    print ("trying to get portapper connection for host " + host)
    pmap = Portmap(host, timeout=timeout)
    pmap.connect()
    print ("portapper connection established for host " + host)
    mnt_port = pmap.getport(Mount.program, Mount.program_version)
    print ("mount port" + str(mnt_port))

    nlmport = pmap.getport(NLM_PROG, NLM_VERS)
    print ("nlm port" + str(nlmport))
    nlmport = 4045
    print ("nlm port(forceset)" + str(nlmport))
    rpc = RPC(host=host, port=nlmport, timeout=timeout)
    rpc.connect()
    data = socket.inet_aton(clientNodeIp) # b'\n\x80\x00\x1b
    print (data)
    res = rpc.request(CUSTOM_NLM_PROGRAM, CUSTOM_NLM_PROG_VERS, CUSTOM_NLM_PROC_VERS, data=data, auth=None)
    print ("rpc response " + str(res))


if __name__ == "__main__":
  ReleaseLocks()

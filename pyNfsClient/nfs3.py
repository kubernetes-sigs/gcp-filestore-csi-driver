import logging
from functools import wraps
from .utils import LONG, str_to_bytes
from .rpc import RPC
from .pack import nfs_pro_v3Packer, nfs_pro_v3Unpacker
from .rtypes import (nfs_fh3, set_uint32, set_uint64, sattr3, set_time, diropargs3, setattr3args, create3args,
                     mkdir3args, symlink3args, commit3args, sattrguard3, access3args, readdir3args, readdirplus3args,
                     read3args, write3args, createhow3, symlinkdata3, mknod3args, mknoddata3, devicedata3, specdata3,
                     link3args, rename3args, nfstime3)
from .const import (NFS3_PROCEDURE_NULL, NFS3_PROCEDURE_GETATTR, NFS3_PROCEDURE_SETATTR, NFS3_PROCEDURE_LOOKUP,
                    NFS3_PROCEDURE_ACCESS, NFS3_PROCEDURE_READLINK, NFS3_PROCEDURE_READ, NFS3_PROCEDURE_WRITE,
                    NFS3_PROCEDURE_CREATE, NFS3_PROCEDURE_MKDIR, NFS3_PROCEDURE_SYMLINK, NFS3_PROCEDURE_MKNOD,
                    NFS3_PROCEDURE_REMOVE, NFS3_PROCEDURE_RMDIR, NFS3_PROCEDURE_RENAME, NFS3_PROCEDURE_LINK,
                    NFS3_PROCEDURE_READDIR, NFS3_PROCEDURE_READDIRPLUS, NFS3_PROCEDURE_FSSTAT, NFS3_PROCEDURE_FSINFO,
                    NFS3_PROCEDURE_PATHCONF, NFS3_PROCEDURE_COMMIT, NFS_PROGRAM, NFS_V3, NF3BLK, NF3CHR, NF3FIFO,
                    NF3SOCK, time_how, DONT_CHANGE, SET_TO_CLIENT_TIME, SET_TO_SERVER_TIME)

logger = logging.getLogger(__package__)


class NFSAccessError(Exception):
    pass


def fh_check(function):
    @wraps(function)
    def check_fh(*args, **kwargs):
        logger.debug("Checking if first argument is bytes type as file/directory handler for [%s]" % function.__name__)
        fh = None
        if len(args) > 1:
            fh = args[1]
        else:
            for k in kwargs:
                if k.endswith("_handle"):
                    fh = kwargs.get(k)
                    break
        if fh and not isinstance(fh, bytes):
            raise TypeError("file/directory should be bytes")
        else:
            return function(*args, **kwargs)
    return check_fh


class NFSv3(RPC):
    def __init__(self, host, port, timeout, auth):
        super(NFSv3, self).__init__(host=host, port=port, timeout=timeout)
        self.auth = auth

    def nfs_request(self, procedure, args, auth):
        return super(NFSv3, self).request(NFS_PROGRAM, NFS_V3, procedure, data=args, auth=auth)

    def null(self):
        logger.debug("NFSv3 procedure %d: NULL on %s" % (NFS3_PROCEDURE_NULL, self.host))
        super(NFSv3, self).request(NFS_PROGRAM, NFS_V3, NFS3_PROCEDURE_NULL)

        return {"status": 0, "resok": None}

    @classmethod
    def get_sattr3(cls, mode=None, uid=None, gid=None, size=None, atime_flag=None, atime_s=0, atime_ns=0,
                   mtime_flag=None, mtime_s=0, mtime_ns=0):
        if atime_flag not in time_how:
            raise ValueError("atime flag must be one of %s" % time_how.keys())

        if mtime_flag not in time_how:
            raise ValueError("mtime flag must be one of %s" % time_how.keys())

        attrs = sattr3(mode=set_uint32(True, int(mode)) if mode is not None else set_uint32(False),
                       uid=set_uint32(True, int(uid)) if uid is not None else set_uint32(False),
                       gid=set_uint32(True, int(gid)) if gid is not None else set_uint32(False),
                       size=set_uint64(True, LONG(size)) if size is not None else set_uint64(False),
                       atime=set_time(SET_TO_CLIENT_TIME, nfstime3(int(atime_s), int(atime_ns)))
                             if atime_flag == SET_TO_CLIENT_TIME else set_time(atime_flag),
                       mtime=set_time(SET_TO_CLIENT_TIME, nfstime3(int(mtime_s), int(mtime_ns)))
                             if mtime_flag == SET_TO_CLIENT_TIME else set_time(mtime_flag))
        return attrs

    @fh_check
    def getattr(self, file_handle, auth=None):
        packer = nfs_pro_v3Packer()
        packer.pack_fhandle3(file_handle)

        logger.debug("NFSv3 procedure %d: GETATTR on %s" % (NFS3_PROCEDURE_GETATTR, self.host))
        data = self.nfs_request(NFS3_PROCEDURE_GETATTR, packer.get_buffer(), auth if auth else self.auth)

        unpacker = nfs_pro_v3Unpacker(data)
        return unpacker.unpack_getattr3res()

    @fh_check
    def setattr(self, file_handle, mode=None, uid=None, gid=None, size=None,
                atime_flag=SET_TO_SERVER_TIME, atime_s=None, atime_us=None,
                mtime_flag=SET_TO_SERVER_TIME, mtime_s=None, mtime_us=None,
                check=False, obj_ctime=None, auth=None):
        packer = nfs_pro_v3Packer()
        attrs = self.get_sattr3(mode, uid, gid, size, atime_flag, atime_s, atime_us, mtime_flag, mtime_s, mtime_us)
        packer.pack_setattr3args(setattr3args(object=nfs_fh3(file_handle),
                                              new_attributes=attrs,
                                              guard=sattrguard3(check=check, ctime=obj_ctime)))

        logger.debug("NFSv3 procedure %d: GETATTR on %s" % (NFS3_PROCEDURE_SETATTR, self.host))
        data = self.nfs_request(NFS3_PROCEDURE_SETATTR, packer.get_buffer(), auth if auth else self.auth)

        unpacker = nfs_pro_v3Unpacker(data)
        return unpacker.unpack_setattr3res()

    @fh_check
    def lookup(self, dir_handle, file_folder, auth=None):
        packer = nfs_pro_v3Packer()
        packer.pack_diropargs3(diropargs3(dir=nfs_fh3(dir_handle), name=str_to_bytes(file_folder)))

        logger.debug("NFSv3 procedure %d: LOOKUP on %s" % (NFS3_PROCEDURE_LOOKUP, self.host))
        data = self.nfs_request(NFS3_PROCEDURE_LOOKUP, packer.get_buffer(), auth if auth else self.auth)

        unpacker = nfs_pro_v3Unpacker(data)
        return unpacker.unpack_lookup3res(data_format='json')

    @fh_check
    def access(self, file_handle, access_option, auth=None):
        packer = nfs_pro_v3Packer()
        packer.pack_access3args(access3args(object=nfs_fh3(file_handle), access=access_option))

        logger.debug("NFSv3 procedure %d: ACCESS on %s" % (NFS3_PROCEDURE_ACCESS, self.host))
        data = self.nfs_request(NFS3_PROCEDURE_ACCESS, packer.get_buffer(), auth if auth else self.auth)

        unpacker = nfs_pro_v3Unpacker(data)
        return unpacker.unpack_access3res()

    @fh_check
    def readlink(self, file_handle, auth=None):
        packer = nfs_pro_v3Packer()
        packer.pack_fhandle3(file_handle)

        logger.debug("NFSv3 procedure %d: READLINK on %s" % (NFS3_PROCEDURE_READLINK, self.host))
        data = self.nfs_request(NFS3_PROCEDURE_READLINK, packer.get_buffer(), auth if auth else self.auth)

        unpacker = nfs_pro_v3Unpacker(data)
        return unpacker.unpack_readlink3res()

    @fh_check
    def read(self, file_handle, offset=0, chunk_count=1024 * 1024, auth=None):
        packer = nfs_pro_v3Packer()
        packer.pack_read3args(read3args(file=nfs_fh3(file_handle), offset=offset, count=chunk_count))

        logger.debug("NFSv3 procedure %d: READ on %s" % (NFS3_PROCEDURE_READ, self.host))
        data = self.nfs_request(NFS3_PROCEDURE_READ, packer.get_buffer(), auth if auth else self.auth)

        unpacker = nfs_pro_v3Unpacker(data)
        return unpacker.unpack_read3res()

    @fh_check
    def write(self, file_handle, offset, count, content, stable_how, auth=None):
        packer = nfs_pro_v3Packer()
        packer.pack_write3args(write3args(file=nfs_fh3(file_handle),
                                          offset=offset,
                                          count=count,
                                          stable=stable_how,
                                          data=str_to_bytes(content)))

        logger.debug("NFSv3 procedure %d: WRITE on %s" % (NFS3_PROCEDURE_WRITE, self.host))
        res = self.nfs_request(NFS3_PROCEDURE_WRITE, packer.get_buffer(), auth if auth else self.auth)

        unpacker = nfs_pro_v3Unpacker(res)
        return unpacker.unpack_write3res()

    @fh_check
    def create(self, dir_handle, file_name, create_mode, mode=None, uid=None, gid=None, size=None,
               atime_flag=SET_TO_SERVER_TIME, atime_s=None, atime_us=None,
                mtime_flag=SET_TO_SERVER_TIME, mtime_s=None, mtime_us=None,
               verf='0', auth=None):
        packer = nfs_pro_v3Packer()
        attrs = self.get_sattr3(mode, uid, gid, size, atime_flag, atime_s, atime_us, mtime_flag, mtime_s, mtime_us)
        packer.pack_create3args(create3args(where=diropargs3(dir=nfs_fh3(dir_handle), name=str_to_bytes(file_name)),
                                            how=createhow3(mode=create_mode, obj_attributes=attrs, verf=verf)))

        logger.debug("NFSv3 procedure %d: CREATE on %s" % (NFS3_PROCEDURE_CREATE, self.host))
        data = self.nfs_request(NFS3_PROCEDURE_CREATE, packer.get_buffer(), auth if auth else self.auth)

        unpacker = nfs_pro_v3Unpacker(data)
        return unpacker.unpack_create3res()

    @fh_check
    def mkdir(self, dir_handle, dir_name, mode=None, uid=None, gid=None,
              atime_flag=SET_TO_SERVER_TIME, atime_s=None, atime_us=None,
              mtime_flag=SET_TO_SERVER_TIME, mtime_s=None, mtime_us=None,
              auth=None):
        packer = nfs_pro_v3Packer()
        attrs = self.get_sattr3(mode, uid, gid, None, atime_flag, atime_s, atime_us, mtime_flag, mtime_s, mtime_us)
        packer.pack_mkdir3args(mkdir3args(where=diropargs3(dir=nfs_fh3(dir_handle), name=str_to_bytes(dir_name)),
                                          attributes=attrs))

        logger.debug("NFSv3 procedure %d: MKDIR on %s" % (NFS3_PROCEDURE_MKDIR, self.host))
        res = self.nfs_request(NFS3_PROCEDURE_MKDIR, packer.get_buffer(), auth if auth else self.auth)

        unpacker = nfs_pro_v3Unpacker(res)
        return unpacker.unpack_mkdir3res()

    @fh_check
    def symlink(self, dir_handle, link_name, link_to_path, auth=None):
        packer = nfs_pro_v3Packer()
        attrs = self.get_sattr3(mode=None, size=None, uid=None, gid=None, atime_flag=DONT_CHANGE, mtime_flag=DONT_CHANGE)
        packer.pack_symlink3args(symlink3args(where=diropargs3(dir=nfs_fh3(dir_handle),
                                                               name=str_to_bytes(link_name)),
                                              symlink=symlinkdata3(symlink_attributes=attrs,
                                                                   symlink_data=str_to_bytes(link_to_path))))

        logger.debug("NFSv3 procedure %d: SYMLINK on %s" % (NFS3_PROCEDURE_SYMLINK, self.host))
        res = self.nfs_request(NFS3_PROCEDURE_SYMLINK, packer.get_buffer(), auth if auth else self.auth)

        unpacker = nfs_pro_v3Unpacker(res)
        return unpacker.unpack_symlink3res()

    @fh_check
    def mknod(self, dir_handle, file_name, ftype,
              mode=None, uid=None, gid=None,
              atime_flag=SET_TO_SERVER_TIME, atime_s=None, atime_us=None,
              mtime_flag=SET_TO_SERVER_TIME, mtime_s=None, mtime_us=None,
              spec_major=0, spec_minor=0, auth=None):
        packer = nfs_pro_v3Packer()
        attrs = self.get_sattr3(mode, uid, gid, None, atime_flag, atime_s, atime_us, mtime_flag, mtime_s, mtime_us)
        if ftype in (NF3CHR, NF3BLK):
            spec = specdata3(major=spec_major, minor=spec_minor)
            what = mknoddata3(type=ftype, device=devicedata3(dev_attributes=attrs, spec=spec))
        elif ftype in (NF3SOCK, NF3FIFO):
            what = mknoddata3(type=ftype, pipe_attributes=attrs)
        else:
            raise ValueError("ftype must be one of [%d, %d, %d, %d]" % (NF3CHR, NF3BLK, NF3SOCK, NF3FIFO))
        packer.pack_mknod3args(mknod3args(where=diropargs3(dir=nfs_fh3(dir_handle),
                                                           name=str_to_bytes(file_name)),
                                          what=what))

        logger.debug("NFSv3 procedure %d: MKNOD on %s" % (NFS3_PROCEDURE_MKNOD, self.host))
        res = self.nfs_request(NFS3_PROCEDURE_MKNOD, packer.get_buffer(), auth if auth else self.auth)

        unpacker = nfs_pro_v3Unpacker(res)
        return unpacker.unpack_mknod3res()

    @fh_check
    def remove(self, dir_handle, file_name, auth=None):
        packer = nfs_pro_v3Packer()
        packer.pack_diropargs3(diropargs3(dir=nfs_fh3(dir_handle), name=str_to_bytes(file_name)))

        logger.debug("NFSv3 procedure %d: REMOVE on %s" % (NFS3_PROCEDURE_REMOVE, self.host))
        res = self.nfs_request(NFS3_PROCEDURE_REMOVE, packer.get_buffer(), auth if auth else self.auth)

        unpacker = nfs_pro_v3Unpacker(res)
        return unpacker.unpack_remove3res()

    @fh_check
    def rmdir(self, dir_handle, dir_name, auth=None):
        packer = nfs_pro_v3Packer()
        packer.pack_diropargs3(diropargs3(dir=nfs_fh3(dir_handle), name=str_to_bytes(dir_name)))

        logger.debug("NFSv3 procedure %d: RMDIR on %s" % (NFS3_PROCEDURE_RMDIR, self.host))
        res = self.nfs_request(NFS3_PROCEDURE_RMDIR, packer.get_buffer(), auth if auth else self.auth)

        unpacker = nfs_pro_v3Unpacker(res)
        return unpacker.unpack_rmdir3res()

    @fh_check
    def rename(self, dir_handle_from, from_name, dir_handle_to, to_name, auth=None):
        if not isinstance(dir_handle_to, bytes):
            raise TypeError("file handle should be bytes")

        packer = nfs_pro_v3Packer()
        packer.pack_rename3args(rename3args(from_v=diropargs3(dir=nfs_fh3(dir_handle_from),
                                                              name=str_to_bytes(from_name)),
                                            to=diropargs3(dir=nfs_fh3(dir_handle_to),
                                                          name=str_to_bytes(to_name))))

        logger.debug("NFSv3 procedure %d: RENAME on %s" % (NFS3_PROCEDURE_RENAME, self.host))
        res = self.nfs_request(NFS3_PROCEDURE_RENAME, packer.get_buffer(), auth if auth else self.auth)

        unpacker = nfs_pro_v3Unpacker(res)
        return unpacker.unpack_rename3res()

    @fh_check
    def link(self, file_handle, link_to_dir_handle, link_name, auth=None):
        packer = nfs_pro_v3Packer()
        packer.pack_link3args(link3args(file=nfs_fh3(file_handle),
                                        link=diropargs3(dir=nfs_fh3(link_to_dir_handle), name=str_to_bytes(link_name))))

        logger.debug("NFSv3 procedure %d: LINK on %s" % (NFS3_PROCEDURE_LINK, self.host))
        res = self.nfs_request(NFS3_PROCEDURE_LINK, packer.get_buffer(), auth if auth else self.auth)

        unpacker = nfs_pro_v3Unpacker(res)
        return unpacker.unpack_link3res()

    @fh_check
    def readdir(self, dir_handle, cookie=0, cookie_verf='0', count=4096, auth=None):
        packer = nfs_pro_v3Packer()
        packer.pack_readdir3args(readdir3args(dir=nfs_fh3(dir_handle),
                                              cookie=cookie,
                                              cookieverf=str_to_bytes(cookie_verf),
                                              count=count))

        logger.debug("NFSv3 procedure %d: READDIR on %s" % (NFS3_PROCEDURE_READDIR, self.host))
        res = self.nfs_request(NFS3_PROCEDURE_READDIR, packer.get_buffer(), auth if auth else self.auth)

        unpacker = nfs_pro_v3Unpacker(res)
        return unpacker.unpack_readdir3res()

    @fh_check
    def readdirplus(self, dir_handle, cookie=0, cookie_verf='0', dircount=4096, maxcount=32768, auth=None):
        packer = nfs_pro_v3Packer()
        packer.pack_readdirplus3args(readdirplus3args(dir=nfs_fh3(dir_handle),
                                                      cookie=cookie,
                                                      cookieverf=str_to_bytes(cookie_verf),
                                                      dircount=dircount,
                                                      maxcount=maxcount))

        logger.debug("NFSv3 procedure %d: READDIRPLUS on %s" % (NFS3_PROCEDURE_READDIRPLUS, self.host))
        res = self.nfs_request(NFS3_PROCEDURE_READDIRPLUS, packer.get_buffer(), auth if auth else self.auth)

        unpacker = nfs_pro_v3Unpacker(res)
        return unpacker.unpack_readdirplus3res()

    @fh_check
    def fsstat(self, file_handle, auth=None):
        packer = nfs_pro_v3Packer()
        packer.pack_fhandle3(file_handle)

        logger.debug("NFSv3 procedure %d: FSSTAT on %s" % (NFS3_PROCEDURE_FSSTAT, self.host))
        data = self.nfs_request(NFS3_PROCEDURE_FSSTAT, packer.get_buffer(), auth if auth else self.auth)

        unpacker = nfs_pro_v3Unpacker(data)
        return unpacker.unpack_fsstat3res()

    @fh_check
    def fsinfo(self, file_handle, auth=None):
        packer = nfs_pro_v3Packer()
        packer.pack_fhandle3(file_handle)

        logger.debug("NFSv3 procedure %d: FSINFO on %s" % (NFS3_PROCEDURE_FSINFO, self.host))
        data = self.nfs_request(NFS3_PROCEDURE_FSINFO, packer.get_buffer(), auth if auth else self.auth)

        unpacker = nfs_pro_v3Unpacker(data)
        return unpacker.unpack_fsinfo3res()

    @fh_check
    def pathconf(self, file_handle, auth=None):
        packer = nfs_pro_v3Packer()
        packer.pack_fhandle3(file_handle)

        logger.debug("NFSv3 procedure %d: PATHCONF on %s" % (NFS3_PROCEDURE_PATHCONF, self.host))
        data = self.nfs_request(NFS3_PROCEDURE_PATHCONF, packer.get_buffer(), auth if auth else self.auth)

        unpacker = nfs_pro_v3Unpacker(data)
        return unpacker.unpack_pathconf3res()

    @fh_check
    def commit(self, file_handle, count=0, offset=0, auth=None):
        packer = nfs_pro_v3Packer()
        packer.pack_commit3args(commit3args(file=nfs_fh3(file_handle), offset=offset, count=count))

        logger.debug("NFSv3 procedure %d: COMMIT on %s" % (NFS3_PROCEDURE_COMMIT, self.host))
        res = self.nfs_request(NFS3_PROCEDURE_COMMIT, packer.get_buffer(), auth if auth else self.auth)

        unpacker = nfs_pro_v3Unpacker(res)
        return unpacker.unpack_commit3res()

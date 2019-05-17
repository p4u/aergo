/**
 * @file    gen_md.c
 * @copyright defined in aergo/LICENSE.txt
 */

#include "common.h"

#include "util.h"
#include "gen_fn.h"
#include "gen_util.h"
#include "syslib.h"

#include "gen_md.h"

#define WASM_MEM_UNIT       65536
#define WASM_MAX_LEN        1024 * 1024

static void
env_gen(gen_t *gen, ir_md_t *md)
{
    int i;
    char qname[NAME_MAX_LEN * 2 + 2];
    vector_t *abis = &md->abis;
    ir_sgmt_t *sgmt = &md->sgmt;
    BinaryenExpressionRef *addrs;

    if (sgmt->offset >= gen->flag.stack_size)
        FATAL(ERROR_STACK_OVERFLOW, gen->flag.stack_size, sgmt->offset);

    addrs = xmalloc(sizeof(BinaryenExpressionRef) * sgmt->size);

    for (i = 0; i < sgmt->size; i++) {
        addrs[i] = i32_gen(gen, sgmt->addrs[i]);
    }

    /* Temporarily maximum memory is limited to 64MB. If the Fee system is introduced in the
     * future, this limitation will be changed. */
    ASSERT1(sgmt->offset / WASM_MEM_UNIT + 1 <= 1024, sgmt->offset);

    BinaryenSetMemory(gen->module, 1, 1024, NULL, (const char **)sgmt->datas, addrs, sgmt->lens,
                      sgmt->size, 0);

    BinaryenAddGlobal(gen->module, "__STACK_TOP", BinaryenTypeInt32(), 1,
                      i32_gen(gen, ALIGN64(sgmt->offset)));
    BinaryenAddGlobal(gen->module, "__STACK_MAX", BinaryenTypeInt32(), 0,
                      i32_gen(gen, gen->flag.stack_size));

    BinaryenAddGlobalExport(gen->module, "__STACK_TOP", "__STACK_TOP");
    BinaryenAddGlobalExport(gen->module, "__STACK_MAX", "__STACK_MAX");

    syslib_gen(gen, FN_ALLOCA);

    vector_foreach(abis, i) {
        ir_abi_t *abi = vector_get_abi(abis, i);

        snprintf(qname, sizeof(qname), "%s.%s", abi->module, abi->name);

        BinaryenAddFunctionImport(gen->module, qname, abi->module, abi->name, abi_gen(gen, abi));
    }

    BinaryenAddMemoryImport(gen->module, NULL, SYSLIB_MODULE, "memory", 0);
}

void
md_gen(gen_t *gen, ir_md_t *md)
{
    int i, n;
    int buf_sz = WASM_MAX_LEN * 2;
    char *buf = xmalloc(buf_sz);

    gen->module = BinaryenModuleCreate();
    gen->md = md;

    if (is_flag_on(gen->flag, FLAG_DEBUG)) {
        char *ptr = strrchr(gen->flag.path, PATH_DELIM);

        md->fno = BinaryenModuleAddDebugInfoFileName(gen->module, ptr ? ptr + 1 : gen->flag.path);
        md->dis = vector_new();
    }

    /* env_gen() is executed after fn_gen() because abi can be added in the process of generating
     * a function and the usage of data segment can also be changed. */

    vector_foreach(&md->fns, i) {
        fn_gen(gen, vector_get_fn(&md->fns, i));
    }

    env_gen(gen, md);

    if (is_flag_on(gen->flag, FLAG_DEBUG)) {
        BinaryenSetDebugInfo(1);
    }
    else if (gen->flag.opt_lvl > 0) {
        ASSERT(BinaryenModuleValidate(gen->module));

        BinaryenSetOptimizeLevel(gen->flag.opt_lvl);
        BinaryenModuleOptimize(gen->module);
    }

    if (is_flag_on(gen->flag, FLAG_DUMP_WAT))
        BinaryenModulePrint(gen->module);

    ASSERT(BinaryenModuleValidate(gen->module));

    n = BinaryenModuleWrite(gen->module, buf, buf_sz);
    if (n <= WASM_MAX_LEN) {
        char path[PATH_MAX_LEN + 1];

        ASSERT(md->name != NULL);

        snprintf(path, sizeof(path), "./%s.wasm", md->name);

        write_file(path, buf, n);
    }
    else {
        /* For now, the binary size is limited to 1MB. */
        FATAL(ERROR_BINARY_OVERFLOW, WASM_MAX_LEN, n);
    }

    BinaryenModuleDispose(gen->module);

    gen->module = NULL;
    gen->md = NULL;
}

/* end of gen_md.c */
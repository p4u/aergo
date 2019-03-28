/**
 * @file    syslib.c
 * @copyright defined in aergo/LICENSE.txt
 */

#include "common.h"

#include "binaryen-c.h"
#include "ast_id.h"
#include "ast_exp.h"
#include "parse.h"
#include "ir_abi.h"
#include "ir_md.h"
#include "gen_util.h"
#include "iobuf.h"

#include "syslib.h"

char *lib_src =
"library system {\n"
"    func abs32(int32 i) int32 : \"abs32\";\n"
"    func abs64(int64 i) int64 : \"abs64\";\n"
"    func abs128(int128 i) int128 : \"mpz_abs\";\n"

"    func pow32(int32 x, int32 y) int32 : \"pow32\";\n"
"    func pow64(int64 x, int32 y) int64 : \"pow64\";\n"
"    func pow128(int128 x, int32 y) int128 : \"mpz_pow_ui\";\n"

"    func sqrt32(int32 x) int32 : \"sqrt32\";\n"
"    func sqrt64(int64 x) int64 : \"sqrt64\";\n"
"    func sqrt128(int128 x) int128 : \"mpz_sqrt\";\n"
"}";

sys_fn_t sys_fntab_[FN_MAX] = {
    { "malloc", SYSLIB_MODULE".malloc", 1, { TYPE_UINT32 }, TYPE_UINT32 },
    { "memcpy", SYSLIB_MODULE".memcpy", 3,
        { TYPE_UINT32, TYPE_UINT32, TYPE_UINT32 }, TYPE_VOID },
    { "strcat", SYSLIB_MODULE".strcat", 2, { TYPE_UINT32, TYPE_UINT32 }, TYPE_UINT32 },
    { "strcmp", SYSLIB_MODULE".strcmp", 2, { TYPE_UINT32, TYPE_UINT32 }, TYPE_UINT32 },
    { "atoi32", SYSLIB_MODULE".atoi32", 1, { TYPE_UINT32 }, TYPE_UINT32 },
    { "atoi64", SYSLIB_MODULE".atoi64", 1, { TYPE_UINT32 }, TYPE_UINT64 },
    { "itoa32", SYSLIB_MODULE".itoa32", 1, { TYPE_UINT32 }, TYPE_UINT32 },
    { "itoa64", SYSLIB_MODULE".itoa64", 1, { TYPE_UINT64 }, TYPE_UINT32 },
    { "mpz_get_i32", SYSLIB_MODULE".mpz_get_i32", 1, { TYPE_UINT32 }, TYPE_UINT32 },
    { "mpz_get_i64", SYSLIB_MODULE".mpz_get_i64", 1, { TYPE_UINT32 }, TYPE_UINT64 },
    { "mpz_get_str", SYSLIB_MODULE".mpz_get_str", 1, { TYPE_UINT32 }, TYPE_UINT32 },
    { "mpz_set_i32", SYSLIB_MODULE".mpz_set_i32", 2,
        { TYPE_UINT32, TYPE_UINT32 }, TYPE_UINT32 },
    { "mpz_set_i64", SYSLIB_MODULE".mpz_set_i64", 2,
        { TYPE_UINT64, TYPE_UINT32 }, TYPE_UINT32 },
    { "mpz_set_str", SYSLIB_MODULE".mpz_set_str", 1, { TYPE_UINT32 }, TYPE_UINT32 },
    { "mpz_add", SYSLIB_MODULE".mpz_add", 2, { TYPE_UINT32, TYPE_UINT32 }, TYPE_UINT32 },
    { "mpz_sub", SYSLIB_MODULE".mpz_sub", 2, { TYPE_UINT32, TYPE_UINT32 }, TYPE_UINT32 },
    { "mpz_mul", SYSLIB_MODULE".mpz_mul", 2, { TYPE_UINT32, TYPE_UINT32 }, TYPE_UINT32 },
    { "mpz_div", SYSLIB_MODULE".mpz_div", 2, { TYPE_UINT32, TYPE_UINT32 }, TYPE_UINT32 },
    { "mpz_mod", SYSLIB_MODULE".mpz_mod", 2, { TYPE_UINT32, TYPE_UINT32 }, TYPE_UINT32 },
    { "mpz_and", SYSLIB_MODULE".mpz_and", 2, { TYPE_UINT32, TYPE_UINT32 }, TYPE_UINT32 },
    { "mpz_or", SYSLIB_MODULE".mpz_or", 2, { TYPE_UINT32, TYPE_UINT32 }, TYPE_UINT32 },
    { "mpz_xor", SYSLIB_MODULE".mpz_xor", 2, { TYPE_UINT32, TYPE_UINT32 }, TYPE_UINT32 },
    { "mpz_rshift", SYSLIB_MODULE".mpz_rshift", 2,
        { TYPE_UINT32, TYPE_UINT32 }, TYPE_UINT32 },
    { "mpz_lshift", SYSLIB_MODULE".mpz_lshift", 2,
        { TYPE_UINT32, TYPE_UINT32 }, TYPE_UINT32 },
    { "mpz_cmp", SYSLIB_MODULE".mpz_cmp", 2,
        { TYPE_UINT32, TYPE_UINT32 }, TYPE_UINT32 },
    { "mpz_neg", SYSLIB_MODULE".mpz_neg", 1, { TYPE_UINT32 }, TYPE_UINT32 },
};

void
syslib_load(ast_t *ast)
{
    flag_t flag = { FLAG_NONE, 0, 0 };
    iobuf_t src;

    iobuf_init(&src, "system_library");
    iobuf_set(&src, strlen(lib_src), lib_src);

    parse(&src, flag, ast);
}

ir_abi_t *
syslib_abi(sys_fn_t *sys_fn)
{
    int i;
    ir_abi_t *abi = xcalloc(sizeof(ir_abi_t));

    ASSERT(sys_fn != NULL);

    abi->module = SYSLIB_MODULE;
    abi->name = sys_fn->name;

    abi->param_cnt = sys_fn->param_cnt;
    abi->params = xmalloc(sizeof(BinaryenType) * abi->param_cnt);

    for (i = 0; i < abi->param_cnt; i++) {
        abi->params[i] = type_gen(sys_fn->params[i]);
    }

    abi->result = type_gen(sys_fn->result);

    return abi;
}

ast_exp_t *
syslib_new_malloc(trans_t *trans, uint32_t size, src_pos_t *pos)
{
    ast_exp_t *res_exp;
    ast_exp_t *param_exp;
    vector_t *param_exps = vector_new();
    sys_fn_t *sys_fn = SYS_FN(FN_MALLOC);

    param_exp = exp_new_lit_int(size, pos);
    meta_set_uint32(&param_exp->meta);

    exp_add(param_exps, param_exp);

    res_exp = exp_new_call(false, NULL, param_exps, pos);

    res_exp->u_call.qname = sys_fn->qname;
    meta_set_uint32(&res_exp->meta);

    md_add_imp(trans->md, syslib_abi(sys_fn));

    return res_exp;
}

ast_exp_t *
syslib_new_memcpy(trans_t *trans, ast_exp_t *dest_exp, ast_exp_t *src_exp,
                   uint32_t size, src_pos_t *pos)
{
    ast_exp_t *res_exp;
    ast_exp_t *param_exp;
    vector_t *param_exps = vector_new();
    sys_fn_t *sys_fn = SYS_FN(FN_MEMCPY);

    exp_add(param_exps, dest_exp);
    exp_add(param_exps, src_exp);

    param_exp = exp_new_lit_int(size, pos);
    meta_set_uint32(&param_exp->meta);

    exp_add(param_exps, param_exp);

    res_exp = exp_new_call(false, NULL, param_exps, pos);

    res_exp->u_call.qname = sys_fn->qname;
    meta_set_void(&res_exp->meta);

    md_add_imp(trans->md, syslib_abi(sys_fn));

    return res_exp;
}

BinaryenExpressionRef
syslib_call_1(gen_t *gen, fn_kind_t kind, BinaryenExpressionRef argument)
{
    sys_fn_t *sys_fn = SYS_FN(kind);

    ASSERT2(sys_fn->param_cnt == 1, kind, sys_fn->param_cnt);

    md_add_imp(gen->md, syslib_abi(sys_fn));

    return BinaryenCall(gen->module, sys_fn->qname, &argument, 1,
                        type_gen(sys_fn->result));
}

BinaryenExpressionRef
syslib_call_2(gen_t *gen, fn_kind_t kind, BinaryenExpressionRef left,
              BinaryenExpressionRef right)
{
    sys_fn_t *sys_fn = SYS_FN(kind);
    BinaryenExpressionRef arguments[2] = { left, right };

    ASSERT2(sys_fn->param_cnt == 2, kind, sys_fn->param_cnt);

    md_add_imp(gen->md, syslib_abi(sys_fn));

    return BinaryenCall(gen->module, sys_fn->qname, arguments, 2,
                        type_gen(sys_fn->result));
}

/* end of syslib.c */

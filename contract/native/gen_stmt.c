/**
 * @file    gen_stmt.c
 * @copyright defined in aergo/LICENSE.txt
 */

#include "common.h"

#include "ast_id.h"
#include "gen_exp.h"
#include "gen_blk.h"
#include "gen_util.h"

#include "gen_stmt.h"

static BinaryenExpressionRef
stmt_gen_assign(gen_t *gen, ast_stmt_t *stmt)
{
    ast_exp_t *l_exp = stmt->u_assign.l_exp;
    ast_exp_t *r_exp = stmt->u_assign.r_exp;
    ast_id_t *id;
    meta_t *meta;
    BinaryenExpressionRef var_exp, val_exp;

    if (is_tuple_exp(l_exp)) {
        int i;
        array_t *l_exps, *r_exps;

        if (!is_tuple_exp(r_exp)) {
            ERROR(ERROR_NOT_SUPPORTED, &r_exp->pos);
            return NULL;
        }

        l_exps = l_exp->u_tup.exps;
        r_exps = r_exp->u_tup.exps;

        if (array_size(l_exps) != array_size(r_exps)) {
            ERROR(ERROR_NOT_SUPPORTED, &r_exp->pos);
            return NULL;
        }

        /* XXX */
        for (i = 0; i < array_size(l_exps); i++) {
        }

        return NULL;
    }

    meta = &l_exp->meta;

    var_exp = exp_gen(gen, l_exp, meta, true);

    ASSERT2(BinaryenExpressionGetId(var_exp) == BinaryenConstId(),
            BinaryenExpressionGetId(var_exp), BinaryenConstId());

    val_exp = exp_gen(gen, r_exp, meta, false);

    id = l_exp->id;
    ASSERT(id != NULL);

    if (id->idx >= 0)
        /* var_exp == index of variable */
        return BinaryenSetLocal(gen->module, BinaryenConstGetValueI32(var_exp), val_exp);
    else
        /* var_exp == offset of variable */
        return BinaryenStore(gen->module, meta_size(meta),
                             BinaryenConstGetValueI32(var_exp), 0,
                             gen_i32(gen, id->meta.addr), val_exp, meta_gen(gen, meta));
}

static BinaryenExpressionRef
stmt_gen_if(gen_t *gen, ast_stmt_t *stmt)
{
    ast_exp_t *cond_exp = stmt->u_if.cond_exp;
    BinaryenExpressionRef cond;
    BinaryenExpressionRef if_body, else_body;

    cond = exp_gen(gen, cond_exp, &cond_exp->meta, false);

    if_body = blk_gen(gen, stmt->u_if.if_blk);

    if (stmt->u_if.else_blk != NULL) {
        else_body = blk_gen(gen, stmt->u_if.else_blk);
    }
    else {
        int i, j = 0;
        array_t *elif_stmts = &stmt->u_if.elif_stmts;
        BinaryenExpressionRef *instrs;

        instrs = xmalloc(sizeof(BinaryenExpressionRef) * array_size(elif_stmts));

        for (i = 0; i < array_size(elif_stmts); i++) {
            instrs[j++] = stmt_gen_if(gen, array_get(elif_stmts, i, ast_stmt_t));
        }

        else_body = BinaryenBlock(gen->module, NULL, instrs, j, BinaryenTypeNone());
    }

    return BinaryenIf(gen->module, cond, if_body, else_body);
}

static BinaryenExpressionRef
stmt_gen_loop(gen_t *gen, ast_stmt_t *stmt)
{
    ast_blk_t *blk = stmt->u_loop.blk;

    if (stmt->u_loop.kind == LOOP_FOR) {
        char label[128];
        BinaryenExpressionRef loop_ref;

        if (stmt->u_loop.init_stmt != NULL)
            gen_add_instr(gen, stmt_gen(gen, stmt->u_loop.init_stmt));

        snprintf(label, sizeof(label), "normal_blk_%d", blk->num);

        loop_ref = BinaryenLoop(gen->module, blk->name, blk_gen(gen, blk));

        return BinaryenBlock(gen->module, xstrdup(label), &loop_ref, 1,
                             BinaryenTypeNone());
    }

    /* XXX */
    return NULL;
}

static BinaryenExpressionRef
stmt_gen_switch(gen_t *gen, ast_stmt_t *stmt)
{
    int i;
    char label[NAME_MAX_LEN + 1];
    int instr_cnt;
    BinaryenExpressionRef *instrs;
    BinaryenExpressionRef block_ref = NULL;
    ast_blk_t *blk = stmt->u_sw.blk;
    array_t *case_stmts = &blk->stmts;
    int dflt_idx = -1;

    instr_cnt = gen->instr_cnt;
    instrs = gen->instrs;

    gen->instr_cnt = 0;
    gen->instrs = NULL;

    for (i = array_size(case_stmts) - 1; i >= 0; i--) {
        ast_stmt_t *case_stmt = array_get(case_stmts, i, ast_stmt_t);
        BinaryenExpressionRef cond_ref, break_ref;

        if (case_stmt->u_case.val_exp == NULL) {
            dflt_idx = i;
            continue;
        }

        cond_ref = exp_gen(gen, case_stmt->u_case.val_exp,
                           &case_stmt->u_case.val_exp->meta, false);

        if (i == 0) {
            if (dflt_idx > 0) {
                snprintf(label, sizeof(label), "case_blk_%d",
                         array_get(case_stmts, dflt_idx - 1, ast_stmt_t)->num);

                break_ref = BinaryenBreak(gen->module, label, cond_ref, NULL);
            }
            else {
                break_ref = BinaryenBreak(gen->module, blk->name, cond_ref, NULL);
            }
        }
        else {
            /* previous statement will be inner block */
            snprintf(label, sizeof(label), "case_blk_%d",
                     array_get(case_stmts, i - 1, ast_stmt_t)->num);

            break_ref = BinaryenBreak(gen->module, label, cond_ref, NULL);
        }

        gen_add_instr(gen, break_ref);
    }

    for (i = 0; i < array_size(case_stmts); i++) {
        ast_stmt_t *case_stmt = array_get(case_stmts, i, ast_stmt_t);

        if (block_ref != NULL)
            gen_add_instr(gen, block_ref);

        gen_stmt_array(gen, case_stmt->u_case.stmts);

        snprintf(label, sizeof(label), "case_blk_%d", case_stmt->num);

        block_ref = BinaryenBlock(gen->module, label, gen->instrs, gen->instr_cnt,
                                  BinaryenTypeNone());

        gen->instr_cnt = 0;
        gen->instrs = NULL;
    }

    if (block_ref != NULL)
        gen_add_instr(gen, block_ref);

    block_ref = BinaryenBlock(gen->module, blk->name, gen->instrs, gen->instr_cnt, 
                              BinaryenTypeNone());

    gen->instr_cnt = instr_cnt;
    gen->instrs = instrs;

    return block_ref;
}

static BinaryenExpressionRef
stmt_gen_return(gen_t *gen, ast_stmt_t *stmt)
{
    meta_t *arg_meta;
    ast_exp_t *arg_exp = stmt->u_ret.arg_exp;
    ast_id_t *ret_id;
    array_t *ret_ids = stmt->u_ret.ret_ids;
    BinaryenExpressionRef value;

    if (arg_exp == NULL)
        return BinaryenReturn(gen->module, NULL);

    if (is_tuple_exp(arg_exp)) {
        int i;
        array_t *elem_exps = arg_exp->u_tup.exps;

        for (i = 0; i < array_size(elem_exps); i++) {
            ast_exp_t *elem_exp = array_get(elem_exps, i, ast_exp_t);
            meta_t *elem_meta = &elem_exp->meta;

            value = exp_gen(gen, elem_exp, elem_meta, false);
            ret_id = array_get(ret_ids, i, ast_id_t);

            gen_add_instr(gen,
                BinaryenStore(gen->module, meta_size(elem_meta), elem_meta->offset, 0,
                    BinaryenGetLocal(gen->module, ret_id->idx, BinaryenTypeInt32()),
                    value, meta_gen(gen, elem_meta)));
        }

        return NULL;
    }

    arg_meta = &arg_exp->meta;
    value = exp_gen(gen, arg_exp, arg_meta, false);
    ret_id = array_get(ret_ids, 0, ast_id_t);

    return BinaryenStore(gen->module, meta_size(arg_meta), arg_meta->offset, 0,
                         BinaryenGetLocal(gen->module, ret_id->idx, BinaryenTypeInt32()),
                         value, meta_gen(gen, arg_meta));
}

static BinaryenExpressionRef
stmt_gen_jump(gen_t *gen, ast_stmt_t *stmt)
{
    ast_exp_t *cond_exp = stmt->u_jump.cond_exp;
    BinaryenExpressionRef cond = NULL;

    if (cond_exp != NULL)
        cond = exp_gen(gen, cond_exp, &cond_exp->meta, false);

    return BinaryenBreak(gen->module, stmt->u_jump.label, cond, NULL);
}

static BinaryenExpressionRef
stmt_gen_goto(gen_t *gen, ast_stmt_t *stmt)
{
    return NULL;
}

static BinaryenExpressionRef
stmt_gen_ddl(gen_t *gen, ast_stmt_t *stmt)
{
    return NULL;
}

static BinaryenExpressionRef
stmt_gen_blk(gen_t *gen, ast_stmt_t *stmt)
{
    return blk_gen(gen, stmt->u_blk.blk);
}

BinaryenExpressionRef
stmt_gen(gen_t *gen, ast_stmt_t *stmt)
{
    switch (stmt->kind) {
    case STMT_NULL:
        return BinaryenNop(gen->module);

    case STMT_EXP:
        return exp_gen(gen, stmt->u_exp.exp, &stmt->u_exp.exp->meta, true);

    case STMT_ASSIGN:
        return stmt_gen_assign(gen, stmt);

    case STMT_IF:
        return stmt_gen_if(gen, stmt);

    case STMT_LOOP:
        return stmt_gen_loop(gen, stmt);

    case STMT_SWITCH:
        return stmt_gen_switch(gen, stmt);

    case STMT_RETURN:
        return stmt_gen_return(gen, stmt);

    case STMT_CONTINUE:
    case STMT_BREAK:
        return stmt_gen_jump(gen, stmt);

    case STMT_GOTO:
        return stmt_gen_goto(gen, stmt);

    case STMT_DDL:
        return stmt_gen_ddl(gen, stmt);

    case STMT_BLK:
        return stmt_gen_blk(gen, stmt);

    default:
        ASSERT1(!"invalid statement", stmt->kind);
    }

    return NULL;
}

/* end of gen_stmt.c */

/**
 * @file    gen.c
 * @copyright defined in aergo/LICENSE.txt
 */

#include "common.h"

#include "gen_md.h"

#include "gen.h"

static void
gen_init(gen_t *gen, flag_t flag)
{
    memset(gen, 0x00, sizeof(gen_t));

    gen->flag = flag;
}

void
gen(ir_t *ir, flag_t flag)
{
    int i;
    gen_t gen;

    if (has_error())
        return;

    gen_init(&gen, flag);

    vector_foreach(&ir->mds, i) {
        md_gen(&gen, vector_get_md(&ir->mds, i));
    }
}

/* end of gen.c */

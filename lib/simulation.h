#include <stdlib.h>
#include <stdbool.h>

typedef struct {
    const char* txhash;
    const char* contract;
    unsigned char followcall;
    const char* chain;
    const char* calldata;
    unsigned char instrace;
    const char* value;
    unsigned char is_json;
} OptFFI;

char* get_simulation_json(OptFFI* opt, bool* success);
void free_simulation_json(char* ptr);
#include <stdint.h>
typedef struct {uint64_t device;int usage;int pressed;} ks_key;
int ks_input_open(void);
int ks_input_read(ks_key *out,int capacity);
void ks_input_close(void);

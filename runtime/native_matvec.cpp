// Speedit out Viper — research-only direct GGUF Q4_K / Q6_K linear layer.
// Input matrix row-major; one GGUF superblock encodes 256 weights.
// Build: g++ -O3 -march=native -fopenmp -shared -fPIC -o libviper_matvec.so native_matvec.cpp
#include <cmath>
#include <cstdint>
#include <cstring>
#include <omp.h>
#include <limits>
#include <vector>
static inline float f16(const uint8_t* p) {
    uint16_t bits = uint16_t(p[0]) | (uint16_t(p[1]) << 8);
    uint32_t sign = (uint32_t(bits & 0x8000)) << 16;
    int exp = (bits >> 10) & 31;
    uint32_t frac = bits & 1023, out;
    if (exp == 0) {
        if (frac == 0) out = sign;
        else {
            int e = -14;
            while ((frac & 1024) == 0) { frac <<= 1; e--; }
            out = sign | uint32_t((e + 127) << 23) | ((frac & 1023) << 13);
        }
    } else if (exp == 31) out = sign | 0x7f800000 | (frac << 13);
    else out = sign | (uint32_t(exp - 15 + 127) << 23) | (frac << 13);
    float val; std::memcpy(&val, &out, sizeof(val)); return val;
}
static inline float block_q4(const uint8_t* b, const float* x) {
    const float d=f16(b), dmin=f16(b+2);
    const uint8_t* s=b+4;
    const uint8_t* q=b+16;
    float scales[8],mins[8];
    for(int j=0;j<8;j++) {
        if (j<4) {scales[j]=s[j]&63; mins[j]=s[j+4]&63;}
        else {scales[j]=(s[j+4]&15)|((s[j-4]>>6)<<4);
              mins[j]=(s[j+4]>>4)|((s[j]>>6)<<4);}
    }
    float sum=0;
    for(int i=0;i<4;i++){
        const uint8_t* qq=q+32*i;
        float p=0,r=0;
        for(int k=0;k<32;k++) {p+=float(qq[k]&15)*x[i*64+k];r+=float(qq[k]>>4)*x[i*64+32+k];}
        // Grouped to allow scales/mins to be shared across all 32 values
        float a=0,z=0;
        for(int k=0;k<32;k++){a+=x[i*64+k];z+=x[i*64+32+k];}
        sum += d*(scales[2*i]*p+scales[2*i+1]*r) - dmin*(mins[2*i]*a+mins[2*i+1]*z);
    }
    return sum;
}
static inline float block_q6(const uint8_t* b,const float* x){
    const uint8_t* ql=b;
    const uint8_t* qh=b+128;
    const int8_t* sc=reinterpret_cast<const int8_t*>(b+192);
    float d=f16(b+208),sum=0;
    for(int i=0;i<2;i++) {
        const uint8_t* a=ql+64*i;
        const uint8_t* h=qh+32*i;
        for(int k=0;k<32;k++) {
            int q1=int((a[k]&15)|((h[k]&3)<<4))-32;
            int q2=int((a[k+32]&15)|(((h[k]>>2)&3)<<4))-32;
            int q3=int((a[k]>>4)|(((h[k]>>4)&3)<<4))-32;
            int q4=int((a[k+32]>>4)|(((h[k]>>6)&3)<<4))-32;
            int off=i*128;
            sum+=float(sc[i*8+0*2+k/16]*q1)*x[off+k];
            sum+=float(sc[i*8+1*2+k/16]*q2)*x[off+32+k];
            sum+=float(sc[i*8+2*2+k/16]*q3)*x[off+64+k];
            sum+=float(sc[i*8+3*2+k/16]*q4)*x[off+96+k];
        }
    }
    return d*sum;
}
extern "C" int viper_matvec(const uint8_t* w,int quant,int64_t rows,int64_t cols,const float* x,float* out,int threads){
    if(!w||!x||!out||rows<=0||cols<=0||cols%256!=0||(quant!=12&&quant!=14))return -1;
    int blocks=int(cols/256);int bytes=quant==12?144:210;
    #pragma omp parallel for schedule(static) num_threads(threads>0?threads:1)
    for(int64_t row=0;row<rows;row++){
        const uint8_t* base=w+row*blocks*bytes;
        double accum=0.0;
        for(int b=0;b<blocks;b++){
            accum+=quant==12?block_q4(base+b*bytes,x+b*256):block_q6(base+b*bytes,x+b*256);
        }
        out[row]=(float)accum;
    }
    return 0;
}

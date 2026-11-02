"""Synthetic smoke test of an installed open-source FFmpeg denoiser.
Not a substitute for WER/CER or tests with real human speech. Reproducible.
Dependencies: numpy, espeak, ffmpeg; use only synthetic audio (no user data).
"""
import json
import math
import os
import subprocess
import wave
from pathlib import Path

import numpy as np

ROOT=Path(__file__).resolve().parent
SAMPLE_RATE=16000

def wave_read(path):
    with wave.open(str(path),'rb') as f:
        assert f.getnchannels()==1 and f.getframerate()==SAMPLE_RATE and f.getsampwidth()==2
        return np.frombuffer(f.readframes(f.getnframes()),dtype='<i2').astype(np.float64)/32768

def wave_write(path, signal):
    with wave.open(str(path),'wb') as f:
        f.setnchannels(1);f.setsampwidth(2);f.setframerate(SAMPLE_RATE)
        f.writeframes((np.clip(signal,-1,1)*32767).astype('<i2').tobytes())

def run(args):
    subprocess.run(args,check=True,stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL)

def best_si_sdr(reference, observed):
    # Filter buffers may add small delay; evaluate best lag within +/- 100 ms.
    ref=reference
    obs=observed
    x=np.fft.rfft(ref, n=1<<int(math.ceil(math.log2(2*len(ref)))))
    y=np.fft.rfft(obs, n=(len(x)-1)*2)
    cc=np.fft.irfft(y*np.conj(x), n=(len(x)-1)*2)
    candidates=[]
    max_lag=1600
    for lag in range(-max_lag,max_lag+1):
        score=cc[lag] if lag>=0 else cc[len(cc)+lag]
        candidates.append((score,lag))
    _,lag=max(candidates)
    if lag>0: ref=ref[:-lag];obs=obs[lag:]
    if lag<0: ref=ref[-lag:];obs=obs[:lag]
    scale=np.dot(obs,ref)/(np.dot(ref,ref)+1e-20)
    target=ref*scale
    err=obs-target
    value=10*np.log10((np.dot(target,target)+1e-20)/(np.dot(err,err)+1e-20))
    return round(float(value),3),int(lag)

def main():
    try:
        run(['espeak','-v','pt-br','-s','170','-w',str(ROOT/'clean-raw.wav'),
             'Oi, eu gostaria de falar sobre isso na segunda-feira. Não sei se consegui explicar direito.'])
        run(['ffmpeg','-nostdin','-loglevel','error','-y','-i',str(ROOT/'clean-raw.wav'),'-ar',str(SAMPLE_RATE),'-ac','1','-c:a','pcm_s16le',str(ROOT/'clean.wav')])
    except (FileNotFoundError,subprocess.CalledProcessError):
        raise SystemExit('Required espeak and ffmpeg missing')
    reference=wave_read(ROOT/'clean.wav')
    rng=np.random.default_rng(20261102)
    results=[]
    for snr_db in [-5,0,5]:
        noise=rng.standard_normal(len(reference))
        noise*=np.sqrt(np.mean(reference**2))/(10**(snr_db/20)*np.sqrt(np.mean(noise**2)))
        name=f'white_{snr_db:+d}db'
        wave_write(ROOT/f'{name}-raw.wav',reference+noise)
        run(['ffmpeg','-nostdin','-loglevel','error','-y','-i',str(ROOT/f'{name}-raw.wav'),'-af','afftdn=nr=18:nf=-35','-ar',str(SAMPLE_RATE),'-ac','1','-c:a','pcm_s16le',str(ROOT/f'{name}-filtered.wav')])
        before,lag_b=best_si_sdr(reference,wave_read(ROOT/f'{name}-raw.wav'))
        after,lag_a=best_si_sdr(reference,wave_read(ROOT/f'{name}-filtered.wav'))
        results.append({'condition':name,'before_si_sdr_db':before,'after_si_sdr_db':after,'delta_db':round(after-before,3),'delay_before_samples':lag_b,'delay_after_samples':lag_a})
    result={'method':'espeak synthetic Portuguese speech + seeded additive Gaussian noise, ffmpeg afftdn; SI-SDR aligned within 100 ms','results':results,'limits':'Not measured with real noisy conversations, phones, or ASR. SI-SDR is not word error rate. Enhancement may remove speech. No vendor engine evaluated.'}
    (ROOT/'denoise-results.json').write_text(json.dumps(result,ensure_ascii=False,indent=2)+'\n')
    print(json.dumps(result,ensure_ascii=False,indent=2))

if __name__=='__main__':main()

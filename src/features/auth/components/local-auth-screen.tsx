'use client';

import { useState, type FormEvent } from 'react';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Icons } from '@/components/icons';
import { logClientAuditEvent } from '@/lib/observability';
import { useSimamiaAuth } from './local-auth';

type AuthMode = 'login' | 'register';

export function LocalAuthScreen({ lang = 'sw' }: { lang?: 'sw' | 'en' }) {
  const { createAccount, signIn } = useSimamiaAuth();
  const [mode, setMode] = useState<AuthMode>('login');
  const [name, setName] = useState('');
  const [phone, setPhone] = useState('');
  const [password, setPassword] = useState('');
  const [confirmPassword, setConfirmPassword] = useState('');
  const [showPassword, setShowPassword] = useState(false);
  const [rememberMe, setRememberMe] = useState(true);
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState('');
  const english = lang === 'en';
  const registering = mode === 'register';

  const submit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    setError('');
    const localPhone = phone.replace(/\D/g, '').replace(/^255/, '').replace(/^0/, '');
    if (localPhone.length !== 9) {
      setError(
        english
          ? 'Enter a complete 9-digit Tanzania phone number.'
          : 'Weka namba kamili ya simu ya Tanzania yenye tarakimu 9.'
      );
      return;
    }
    if (registering && password !== confirmPassword) {
      setError(english ? 'The passwords do not match.' : 'Manenosiri hayafanani.');
      return;
    }
    setSubmitting(true);
    logClientAuditEvent(registering ? 'auth.account_creation_started' : 'auth.sign_in_started', {
      area: 'auth',
      operation: registering ? 'local_register' : 'local_password'
    });
    try {
      if (registering) {
        await createAccount({
          name: name.trim(),
          phone: `+255${localPhone}`,
          password,
          rememberMe
        });
      } else {
        await signIn({ phone: `+255${localPhone}`, password, rememberMe });
      }
      logClientAuditEvent(
        registering ? 'auth.account_creation_succeeded' : 'auth.sign_in_succeeded',
        {
          area: 'auth',
          operation: registering ? 'local_register' : 'local_password'
        }
      );
    } catch (cause) {
      setError(
        cause instanceof Error
          ? cause.message
          : english
            ? 'Could not connect to the account service. Try again.'
            : 'Imeshindikana kuwasiliana na huduma ya akaunti. Jaribu tena.'
      );
    } finally {
      setSubmitting(false);
    }
  };

  const switchMode = () => {
    setError('');
    setMode(registering ? 'login' : 'register');
    setPassword('');
    setConfirmPassword('');
  };

  return (
    <main className='bg-background flex min-h-svh items-center justify-center p-4'>
      <Card className='w-full max-w-md'>
        <CardHeader className='space-y-4'>
          <div className='flex items-center gap-3'>
            <div className='bg-primary text-primary-foreground flex size-11 shrink-0 items-center justify-center rounded-xl'>
              <Icons.dashboard className='size-5' />
            </div>
            <CardTitle className='text-2xl'>Simamia</CardTitle>
          </div>
          <CardDescription>
            {english
              ? registering
                ? 'Create a local Simamia account with your phone number and password. Your account is stored by the Simamia API you use.'
                : 'Sign in with your local Simamia account to open your business workspace.'
              : registering
                ? 'Fungua akaunti ya Simamia kwa namba ya simu na nenosiri. Akaunti itahifadhiwa na API ya Simamia unayotumia.'
                : 'Ingia kwa akaunti yako ya Simamia ili kufungua eneo la biashara yako.'}
          </CardDescription>
        </CardHeader>
        <CardContent className='space-y-4'>
          <form className='space-y-3' onSubmit={submit}>
            {registering && (
              <div className='space-y-2'>
                <label className='text-sm font-medium' htmlFor='local-name'>
                  {english ? 'Full name' : 'Jina kamili'}
                </label>
                <input
                  id='local-name'
                  name='name'
                  type='text'
                  autoComplete='name'
                  minLength={2}
                  maxLength={80}
                  required
                  value={name}
                  onChange={(event) => setName(event.target.value)}
                  className='border-input bg-background ring-offset-background placeholder:text-muted-foreground focus-visible:ring-ring flex h-10 w-full rounded-md border px-3 py-2 text-sm focus-visible:ring-2 focus-visible:ring-offset-2 focus-visible:outline-none'
                />
              </div>
            )}
            <div className='space-y-2'>
              <label className='text-sm font-medium' htmlFor='local-phone'>
                {english ? 'Phone number' : 'Namba ya simu'}
              </label>
              <div className='border-input bg-background focus-within:ring-ring flex h-10 w-full items-center rounded-md border focus-within:ring-2 focus-within:ring-offset-2'>
                <span className='text-muted-foreground border-r px-3 text-sm' aria-hidden='true'>
                  +255
                </span>
                <input
                  id='local-phone'
                  name='phone'
                  type='tel'
                  inputMode='numeric'
                  autoComplete='tel-national'
                  required
                  value={phone}
                  onChange={(event) =>
                    setPhone(
                      event.target.value
                        .replace(/\D/g, '')
                        .replace(/^255/, '')
                        .replace(/^0/, '')
                        .slice(0, 9)
                    )
                  }
                  placeholder='712 345 678'
                  className='placeholder:text-muted-foreground h-full min-w-0 flex-1 rounded-r-md bg-transparent px-3 py-2 text-sm outline-none'
                />
              </div>
            </div>
            <div className='space-y-2'>
              <label className='text-sm font-medium' htmlFor='local-password'>
                {english ? 'Password' : 'Nenosiri'}
              </label>
              <div className='relative'>
                <input
                  id='local-password'
                  name='password'
                  type={showPassword ? 'text' : 'password'}
                  autoComplete={registering ? 'new-password' : 'current-password'}
                  minLength={registering ? 8 : undefined}
                  maxLength={72}
                  required
                  value={password}
                  onChange={(event) => setPassword(event.target.value)}
                  className='border-input bg-background ring-offset-background placeholder:text-muted-foreground focus-visible:ring-ring flex h-10 w-full rounded-md border px-3 py-2 pr-10 text-sm focus-visible:ring-2 focus-visible:ring-offset-2 focus-visible:outline-none'
                />
                <button
                  type='button'
                  className='text-muted-foreground hover:text-foreground absolute inset-y-0 right-0 flex items-center px-3'
                  onClick={() => setShowPassword((visible) => !visible)}
                  aria-label={
                    showPassword
                      ? english
                        ? 'Hide password'
                        : 'Ficha nenosiri'
                      : english
                        ? 'Show password'
                        : 'Onyesha nenosiri'
                  }
                  aria-pressed={showPassword}
                >
                  {showPassword ? (
                    <Icons.eyeOff className='size-4' />
                  ) : (
                    <Icons.eye className='size-4' />
                  )}
                </button>
              </div>
            </div>
            {registering && (
              <div className='space-y-2'>
                <label className='text-sm font-medium' htmlFor='local-confirm-password'>
                  {english ? 'Confirm password' : 'Thibitisha nenosiri'}
                </label>
                <input
                  id='local-confirm-password'
                  name='confirm-password'
                  type={showPassword ? 'text' : 'password'}
                  autoComplete='new-password'
                  minLength={8}
                  maxLength={72}
                  required
                  value={confirmPassword}
                  onChange={(event) => setConfirmPassword(event.target.value)}
                  className='border-input bg-background ring-offset-background placeholder:text-muted-foreground focus-visible:ring-ring flex h-10 w-full rounded-md border px-3 py-2 text-sm focus-visible:ring-2 focus-visible:ring-offset-2 focus-visible:outline-none'
                />
              </div>
            )}
            <label className='flex cursor-pointer items-center gap-2 text-sm'>
              <input
                type='checkbox'
                checked={rememberMe}
                onChange={(event) => setRememberMe(event.target.checked)}
                className='accent-primary size-4 rounded border'
              />
              {english ? 'Remember me' : 'Nikumbuke'}
            </label>
            <Button className='w-full' size='lg' type='submit' disabled={submitting}>
              {submitting
                ? english
                  ? 'Please wait…'
                  : 'Subiri kidogo…'
                : registering
                  ? english
                    ? 'Create local account'
                    : 'Fungua akaunti ya ndani'
                  : english
                    ? 'Sign in'
                    : 'Ingia'}
            </Button>
          </form>
          <p className='text-muted-foreground text-center text-sm'>
            {registering
              ? english
                ? 'Already have an account?'
                : 'Tayari una akaunti?'
              : english
                ? 'New to Simamia?'
                : 'Mpya kwa Simamia?'}{' '}
            <button
              type='button'
              className='text-primary font-medium underline-offset-4 hover:underline'
              onClick={switchMode}
              disabled={submitting}
            >
              {registering
                ? english
                  ? 'Sign in'
                  : 'Ingia'
                : english
                  ? 'Create account'
                  : 'Fungua akaunti'}
            </button>
          </p>
          {error && (
            <p className='text-destructive text-center text-sm' role='alert'>
              {error}
            </p>
          )}
          <p className='text-muted-foreground text-center text-xs'>
            {english
              ? 'Passwords are stored as salted, one-way hashes by your Simamia API. Use HTTPS when you access it over a network.'
              : 'Nenosiri huhifadhiwa kwa usimbaji wa upande mmoja kwenye API yako ya Simamia. Tumia HTTPS unapounganisha kupitia mtandao.'}
          </p>
        </CardContent>
      </Card>
    </main>
  );
}

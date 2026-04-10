interface ErrorAlertProps {
  message: string;
}

export function ErrorAlert({ message }: ErrorAlertProps) {
  return (
    <div className="error-alert" role="alert">
      {message}
    </div>
  );
}

import {Component, ReactNode} from 'react';

type State = {error: Error | null};

/** Keeps one broken panel from blanking the whole window. */
export default class ErrorBoundary extends Component<{children: ReactNode}, State> {
  state: State = {error: null};

  static getDerivedStateFromError(error: Error): State {
    return {error};
  }

  render() {
    if (!this.state.error) return this.props.children;
    return (
      <div className="page">
        <div className="banner error">
          Màn hình này gặp lỗi: {this.state.error.message}
          <div style={{marginTop: 8}}>
            <button onClick={() => this.setState({error: null})}>Thử lại</button>
          </div>
        </div>
      </div>
    );
  }
}

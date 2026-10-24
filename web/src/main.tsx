import {createRoot} from 'react-dom/client';
import App from './App';
import 'bootstrap-icons/font/bootstrap-icons.css';
import '@fontsource/tajawal/400.css';
import '@fontsource/tajawal/500.css';
import '@fontsource/tajawal/700.css';
import '@fontsource/tajawal/800.css';
import './styles.css';
import {setBootstrapDirection} from './bootstrapStyles';

setBootstrapDirection('rtl');
createRoot(document.getElementById('root')!).render(<App/>);
